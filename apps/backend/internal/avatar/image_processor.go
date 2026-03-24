package avatar

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os/exec"
	"strings"

	_ "image/gif"
	_ "image/jpeg"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const maxDecodedImagePixels int64 = 40_000_000

type commandRunner func(ctx context.Context, binary string, args []string, stdin []byte) (stdout []byte, stderr []byte, err error)

type FFmpegWebPProcessor struct {
	ffmpegBinary string
	targetSize   int
	runCommand   commandRunner
}

func NewFFmpegWebPProcessor(ffmpegBinary string, targetSize int) *FFmpegWebPProcessor {
	binary := strings.TrimSpace(ffmpegBinary)
	if binary == "" {
		binary = "ffmpeg"
	}
	if targetSize <= 0 {
		targetSize = DefaultTargetSize
	}
	return &FFmpegWebPProcessor{
		ffmpegBinary: binary,
		targetSize:   targetSize,
		runCommand:   defaultCommandRunner,
	}
}

func (p *FFmpegWebPProcessor) Normalize(ctx context.Context, raw []byte) ([]byte, error) {
	if p == nil {
		return nil, ErrServiceNotReady
	}
	if len(raw) == 0 {
		return nil, ErrPayloadEmpty
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: decode config: %v", ErrInvalidImage, err)
	}
	if !isSupportedImageFormat(format) {
		return nil, fmt.Errorf("%w: unsupported format %q", ErrInvalidImage, format)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("%w: invalid dimensions", ErrInvalidImage)
	}
	if int64(config.Width)*int64(config.Height) > maxDecodedImagePixels {
		return nil, fmt.Errorf("%w: image dimensions exceed limit", ErrInvalidImage)
	}

	decodedImage, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: decode image: %v", ErrInvalidImage, err)
	}

	normalizedImage := cropAndResizeSquare(decodedImage, p.targetSize)

	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, normalizedImage); err != nil {
		return nil, fmt.Errorf("encode normalized png: %w", err)
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-f", "image2pipe",
		"-vcodec", "png",
		"-i", "pipe:0",
		"-vcodec", "libwebp",
		"-q:v", "82",
		"-compression_level", "6",
		"-preset", "picture",
		"-f", "webp",
		"pipe:1",
	}

	stdout, stderr, err := p.runCommand(ctx, p.ffmpegBinary, args, pngBuffer.Bytes())
	if err != nil {
		trimmedStderr := strings.TrimSpace(string(stderr))
		if trimmedStderr == "" {
			return nil, fmt.Errorf("encode webp with ffmpeg: %w", err)
		}
		return nil, fmt.Errorf("encode webp with ffmpeg: %w (%s)", err, trimmedStderr)
	}
	if len(stdout) == 0 {
		return nil, fmt.Errorf("encode webp with ffmpeg: empty output")
	}

	return stdout, nil
}

func isSupportedImageFormat(format string) bool {
	switch strings.TrimSpace(strings.ToLower(format)) {
	case "jpeg", "jpg", "png", "gif", "webp":
		return true
	default:
		return false
	}
}

func cropAndResizeSquare(src image.Image, targetSize int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	side := width
	if height < side {
		side = height
	}

	offsetX := bounds.Min.X + (width-side)/2
	offsetY := bounds.Min.Y + (height-side)/2
	sourceRect := image.Rect(offsetX, offsetY, offsetX+side, offsetY+side)

	cropped := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(cropped, cropped.Bounds(), src, sourceRect.Min, draw.Src)

	destination := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
	xdraw.CatmullRom.Scale(destination, destination.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)

	return destination
}

func defaultCommandRunner(ctx context.Context, binary string, args []string, stdin []byte) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
