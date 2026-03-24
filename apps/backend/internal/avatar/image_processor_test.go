package avatar

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func createPNGTestImage(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png test image: %v", err)
	}
	return buf.Bytes()
}

func TestFFmpegWebPProcessor_NormalizeSuccess(t *testing.T) {
	processor := NewFFmpegWebPProcessor("ffmpeg", 512)
	if processor == nil {
		t.Fatalf("expected non-nil processor")
	}

	called := false
	processor.runCommand = func(_ context.Context, binary string, args []string, stdin []byte) ([]byte, []byte, error) {
		called = true
		if binary != "ffmpeg" {
			t.Fatalf("unexpected ffmpeg binary: %s", binary)
		}
		if len(stdin) == 0 {
			t.Fatalf("expected png payload stdin")
		}
		if len(args) == 0 || args[len(args)-1] != "pipe:1" {
			t.Fatalf("unexpected ffmpeg args: %v", args)
		}
		return []byte("webp-output"), nil, nil
	}

	out, err := processor.Normalize(context.Background(), createPNGTestImage(t, 800, 400))
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if !called {
		t.Fatalf("expected ffmpeg runner to be called")
	}
	if string(out) != "webp-output" {
		t.Fatalf("unexpected normalize output: %q", string(out))
	}
}

func TestFFmpegWebPProcessor_NormalizeErrors(t *testing.T) {
	processor := NewFFmpegWebPProcessor("ffmpeg", 512)

	if _, err := processor.Normalize(context.Background(), nil); !errors.Is(err, ErrPayloadEmpty) {
		t.Fatalf("expected ErrPayloadEmpty, got %v", err)
	}

	if _, err := processor.Normalize(context.Background(), []byte("not-an-image")); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected ErrInvalidImage for invalid payload, got %v", err)
	}

	processor.runCommand = func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, []byte, error) {
		return nil, []byte("ffmpeg failed"), errors.New("exit status 1")
	}

	_, err := processor.Normalize(context.Background(), createPNGTestImage(t, 100, 100))
	if err == nil {
		t.Fatal("expected ffmpeg error")
	}
	if !strings.Contains(err.Error(), "ffmpeg failed") {
		t.Fatalf("expected stderr in error, got %v", err)
	}
}

func TestCropAndResizeSquare(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 300, 120))
	out := cropAndResizeSquare(src, 512)
	if out.Bounds().Dx() != 512 || out.Bounds().Dy() != 512 {
		t.Fatalf("unexpected output bounds: %+v", out.Bounds())
	}
}
