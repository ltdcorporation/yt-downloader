package heatmap

import (
	"math"
	"sort"
)

const AlgorithmVersion = "v1"

type Point struct {
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Value     float64 `json:"value"`
}

type Meta struct {
	Available        bool   `json:"available"`
	Bins             int    `json:"bins"`
	AlgorithmVersion string `json:"algorithm_version"`
}

type Analyzer struct {
	MaxMoments        int
	ThresholdAlpha    float64
	SmoothingRadius   int
	IntroGuardRatio   float64
	IntroGuardMinSec  float64
	IntroGuardMaxSec  float64
	MinDistanceRatio  float64
	MinDistanceMinSec float64
}

type candidate struct {
	score  float64
	center float64
}

func NewAnalyzer() Analyzer {
	return Analyzer{
		MaxMoments:        8,
		ThresholdAlpha:    0.12,
		SmoothingRadius:   1,
		IntroGuardRatio:   0.03,
		IntroGuardMinSec:  6,
		IntroGuardMaxSec:  30,
		MinDistanceRatio:  0.05,
		MinDistanceMinSec: 3,
	}
}

func Analyze(points []Point, durationSeconds int) ([]Point, []int, Meta) {
	analyzer := NewAnalyzer()
	return analyzer.Analyze(points, durationSeconds)
}

func (a Analyzer) Analyze(points []Point, durationSeconds int) ([]Point, []int, Meta) {
	normalized := normalizePoints(points)
	meta := Meta{
		Available:        len(normalized) > 0,
		Bins:             len(normalized),
		AlgorithmVersion: AlgorithmVersion,
	}
	if len(normalized) == 0 {
		return nil, nil, meta
	}

	values := extractValues(normalized)
	smoothed := smooth(values, a.smoothingRadius())
	threshold := adaptiveThreshold(values, a.thresholdAlpha())
	guardSec := introGuardSeconds(durationSeconds, a)
	minDistanceSec := minDistanceSeconds(durationSeconds, a)

	allCandidates := collectLocalMaxima(normalized, values, smoothed, threshold)
	if len(allCandidates) == 0 {
		fallback := fallbackCandidate(normalized, smoothed, guardSec)
		if fallback == nil {
			return normalized, nil, meta
		}
		moments := []int{toSecond(fallback.center, durationSeconds)}
		return normalized, moments, meta
	}

	candidates := filterCandidatesAfterGuard(allCandidates, guardSec)
	if len(candidates) == 0 {
		candidates = allCandidates
	}

	selected := nonMaximumSuppression(candidates, minDistanceSec, a.maxMoments())
	if len(selected) == 0 {
		fallback := fallbackCandidate(normalized, smoothed, guardSec)
		if fallback == nil {
			return normalized, nil, meta
		}
		moments := []int{toSecond(fallback.center, durationSeconds)}
		return normalized, moments, meta
	}

	moments := make([]int, 0, len(selected))
	seen := make(map[int]struct{}, len(selected))
	for _, item := range selected {
		second := toSecond(item.center, durationSeconds)
		if _, exists := seen[second]; exists {
			continue
		}
		seen[second] = struct{}{}
		moments = append(moments, second)
	}
	sort.Ints(moments)

	return normalized, moments, meta
}

func normalizePoints(points []Point) []Point {
	normalized := make([]Point, 0, len(points))
	for _, point := range points {
		if !isFinite(point.StartTime) || !isFinite(point.EndTime) {
			continue
		}
		start := point.StartTime
		if start < 0 {
			start = 0
		}
		end := point.EndTime
		if end <= start {
			continue
		}
		value := point.Value
		if !isFinite(value) {
			value = 0
		}
		if value < 0 {
			value = 0
		}
		normalized = append(normalized, Point{
			StartTime: start,
			EndTime:   end,
			Value:     value,
		})
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].StartTime < normalized[j].StartTime
	})
	return normalized
}

func extractValues(points []Point) []float64 {
	values := make([]float64, 0, len(points))
	for _, point := range points {
		values = append(values, point.Value)
	}
	return values
}

func smooth(values []float64, radius int) []float64 {
	if len(values) == 0 {
		return nil
	}
	if radius <= 0 {
		out := make([]float64, len(values))
		copy(out, values)
		return out
	}

	out := make([]float64, len(values))
	for idx := range values {
		left := idx - radius
		if left < 0 {
			left = 0
		}
		right := idx + radius
		if right >= len(values) {
			right = len(values) - 1
		}

		sum := 0.0
		count := 0
		for scan := left; scan <= right; scan++ {
			sum += values[scan]
			count++
		}
		if count == 0 {
			continue
		}
		out[idx] = sum / float64(count)
	}
	return out
}

func adaptiveThreshold(values []float64, alpha float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	medianValue := median(values)
	if maxValue <= medianValue {
		return medianValue
	}
	return medianValue + alpha*(maxValue-medianValue)
}

func collectLocalMaxima(points []Point, rawValues []float64, smoothed []float64, threshold float64) []candidate {
	if len(points) == 0 || len(smoothed) == 0 || len(rawValues) != len(smoothed) {
		return nil
	}

	candidates := make([]candidate, 0, len(points)/4)
	for idx := range rawValues {
		score := smoothed[idx]
		if score < threshold {
			continue
		}
		if !isLocalMaximum(rawValues, idx) {
			continue
		}
		center := (points[idx].StartTime + points[idx].EndTime) / 2
		candidates = append(candidates, candidate{score: score, center: center})
	}
	return candidates
}

func isLocalMaximum(values []float64, index int) bool {
	if len(values) == 0 {
		return false
	}
	if len(values) == 1 {
		return true
	}
	if index == 0 {
		return values[0] > values[1]
	}
	if index == len(values)-1 {
		return values[index] > values[index-1]
	}
	return values[index] >= values[index-1] && values[index] > values[index+1]
}

func filterCandidatesAfterGuard(candidates []candidate, guardSec float64) []candidate {
	if guardSec <= 0 {
		return candidates
	}
	filtered := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		if item.center > guardSec {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func nonMaximumSuppression(candidates []candidate, minDistanceSec float64, maxMoments int) []candidate {
	if len(candidates) == 0 {
		return nil
	}
	sortedCandidates := make([]candidate, len(candidates))
	copy(sortedCandidates, candidates)
	sort.SliceStable(sortedCandidates, func(i, j int) bool {
		if sortedCandidates[i].score == sortedCandidates[j].score {
			return sortedCandidates[i].center < sortedCandidates[j].center
		}
		return sortedCandidates[i].score > sortedCandidates[j].score
	})

	selected := make([]candidate, 0, min(maxMoments, len(sortedCandidates)))
	for _, item := range sortedCandidates {
		if len(selected) >= maxMoments {
			break
		}

		blocked := false
		for _, picked := range selected {
			if math.Abs(item.center-picked.center) < minDistanceSec {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		selected = append(selected, item)
	}

	return selected
}

func fallbackCandidate(points []Point, smoothed []float64, guardSec float64) *candidate {
	if len(points) == 0 || len(smoothed) == 0 {
		return nil
	}

	best := -1
	bestScore := -1.0
	for idx := range smoothed {
		center := (points[idx].StartTime + points[idx].EndTime) / 2
		if center <= guardSec {
			continue
		}
		if smoothed[idx] > bestScore {
			bestScore = smoothed[idx]
			best = idx
		}
	}

	if best >= 0 {
		center := (points[best].StartTime + points[best].EndTime) / 2
		return &candidate{score: smoothed[best], center: center}
	}

	best = 0
	bestScore = smoothed[0]
	for idx := 1; idx < len(smoothed); idx++ {
		if smoothed[idx] > bestScore {
			bestScore = smoothed[idx]
			best = idx
		}
	}
	center := (points[best].StartTime + points[best].EndTime) / 2
	return &candidate{score: smoothed[best], center: center}
}

func introGuardSeconds(durationSeconds int, analyzer Analyzer) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	duration := float64(durationSeconds)
	guard := duration * analyzer.introGuardRatio()
	guard = clamp(guard, analyzer.introGuardMin(), analyzer.introGuardMax())
	maxGuard := duration * 0.35
	if guard > maxGuard {
		guard = maxGuard
	}
	if guard < 0 {
		guard = 0
	}
	return guard
}

func minDistanceSeconds(durationSeconds int, analyzer Analyzer) float64 {
	if durationSeconds <= 0 {
		return analyzer.minDistanceMin()
	}
	distance := float64(durationSeconds) * analyzer.minDistanceRatio()
	if distance < analyzer.minDistanceMin() {
		distance = analyzer.minDistanceMin()
	}
	return distance
}

func toSecond(value float64, durationSeconds int) int {
	second := int(math.Round(value))
	if second < 0 {
		second = 0
	}
	if durationSeconds > 0 && second > durationSeconds {
		second = durationSeconds
	}
	return second
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copied := make([]float64, len(values))
	copy(copied, values)
	sort.Float64s(copied)
	middle := len(copied) / 2
	if len(copied)%2 == 1 {
		return copied[middle]
	}
	return (copied[middle-1] + copied[middle]) / 2
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func clamp(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (a Analyzer) maxMoments() int {
	if a.MaxMoments <= 0 {
		return 8
	}
	return a.MaxMoments
}

func (a Analyzer) thresholdAlpha() float64 {
	if a.ThresholdAlpha <= 0 {
		return 0.12
	}
	if a.ThresholdAlpha > 1 {
		return 1
	}
	return a.ThresholdAlpha
}

func (a Analyzer) smoothingRadius() int {
	if a.SmoothingRadius < 0 {
		return 0
	}
	return a.SmoothingRadius
}

func (a Analyzer) introGuardRatio() float64 {
	if a.IntroGuardRatio <= 0 {
		return 0.03
	}
	return a.IntroGuardRatio
}

func (a Analyzer) introGuardMin() float64 {
	if a.IntroGuardMinSec < 0 {
		return 0
	}
	return a.IntroGuardMinSec
}

func (a Analyzer) introGuardMax() float64 {
	if a.IntroGuardMaxSec <= 0 {
		return 30
	}
	return a.IntroGuardMaxSec
}

func (a Analyzer) minDistanceRatio() float64 {
	if a.MinDistanceRatio <= 0 {
		return 0.05
	}
	return a.MinDistanceRatio
}

func (a Analyzer) minDistanceMin() float64 {
	if a.MinDistanceMinSec <= 0 {
		return 3
	}
	return a.MinDistanceMinSec
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
