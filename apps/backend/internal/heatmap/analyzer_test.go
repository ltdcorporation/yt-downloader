package heatmap

import (
	"math"
	"reflect"
	"testing"
)

func makePoints(values []float64, binSize float64) []Point {
	points := make([]Point, 0, len(values))
	cursor := 0.0
	for _, value := range values {
		points = append(points, Point{
			StartTime: cursor,
			EndTime:   cursor + binSize,
			Value:     value,
		})
		cursor += binSize
	}
	return points
}

func TestAnalyze_EmptyInput(t *testing.T) {
	normalized, moments, meta := Analyze(nil, 120)
	if len(normalized) != 0 {
		t.Fatalf("expected no normalized points, got %d", len(normalized))
	}
	if len(moments) != 0 {
		t.Fatalf("expected no key moments, got %v", moments)
	}
	if meta.Available {
		t.Fatal("expected meta available=false")
	}
	if meta.Bins != 0 {
		t.Fatalf("expected bins=0, got %d", meta.Bins)
	}
	if meta.AlgorithmVersion != AlgorithmVersion {
		t.Fatalf("unexpected algorithm version, got=%q want=%q", meta.AlgorithmVersion, AlgorithmVersion)
	}
}

func TestAnalyze_NormalizeInvalidPoints(t *testing.T) {
	input := []Point{
		{StartTime: 10, EndTime: 20, Value: 0.4},
		{StartTime: 5, EndTime: 9, Value: 0.2},
		{StartTime: 20, EndTime: 20, Value: 0.5},          // invalid equal range
		{StartTime: 30, EndTime: 40, Value: math.NaN()},   // invalid value -> coerced to 0
		{StartTime: -5, EndTime: 3, Value: 0.6},           // negative start -> clamped to 0
		{StartTime: math.Inf(1), EndTime: 50, Value: 0.8}, // invalid start
		{StartTime: 60, EndTime: math.Inf(1), Value: 0.8}, // invalid end
		{StartTime: 70, EndTime: 65, Value: 0.8},          // invalid range
		{StartTime: 80, EndTime: 90, Value: math.Inf(-1)}, // invalid value -> coerced to 0
		{StartTime: 100, EndTime: 110, Value: -1},         // negative value -> clamped to 0
	}

	normalized, _, meta := Analyze(input, 120)
	if !meta.Available {
		t.Fatal("expected available=true")
	}
	if meta.Bins != len(normalized) {
		t.Fatalf("expected bins=%d got=%d", len(normalized), meta.Bins)
	}
	if len(normalized) != 6 {
		t.Fatalf("expected 6 normalized points, got %d (%+v)", len(normalized), normalized)
	}
	if normalized[0].StartTime != 0 {
		t.Fatalf("expected first start to be clamped to 0, got %.2f", normalized[0].StartTime)
	}
	for idx := 1; idx < len(normalized); idx++ {
		if normalized[idx-1].StartTime > normalized[idx].StartTime {
			t.Fatalf("points should be sorted by start time: %+v", normalized)
		}
	}
	if normalized[3].Value != 0 {
		t.Fatalf("expected NaN value to be normalized to 0, got %.2f", normalized[3].Value)
	}
}

func TestAnalyze_SkipsIntroPeakWhenBetterPeaksExist(t *testing.T) {
	values := []float64{1.0, 0.35, 0.22, 0.2, 0.3, 0.95, 0.25, 0.88, 0.3, 0.2}
	points := makePoints(values, 10)

	normalized, moments, meta := Analyze(points, 100)
	if !meta.Available {
		t.Fatal("expected available=true")
	}
	if len(normalized) != len(points) {
		t.Fatalf("expected normalized len %d, got %d", len(points), len(normalized))
	}
	if len(moments) < 2 {
		t.Fatalf("expected >=2 key moments, got %v", moments)
	}
	if moments[0] <= 6 {
		t.Fatalf("expected first selected moment outside intro guard, got %v", moments)
	}
	if !reflect.DeepEqual(moments, []int{55, 75}) {
		t.Fatalf("unexpected key moments, got %v want [55 75]", moments)
	}
}

func TestAnalyze_FallbackWhenNoPeakShape(t *testing.T) {
	values := []float64{0.5, 0.5, 0.5, 0.5, 0.5}
	points := makePoints(values, 10)

	_, moments, _ := Analyze(points, 50)
	if len(moments) != 1 {
		t.Fatalf("expected exactly one fallback moment, got %v", moments)
	}
	if moments[0] <= 0 {
		t.Fatalf("fallback moment should be positive second, got %v", moments)
	}
}

func TestAnalyze_MinDistanceSuppression(t *testing.T) {
	values := []float64{0.1, 0.2, 0.9, 0.2, 0.85, 0.2, 0.1, 0.8, 0.2, 0.1}
	points := makePoints(values, 10)

	analyzer := NewAnalyzer()
	analyzer.MinDistanceRatio = 0.35 // duration=100 => min distance = 35s

	_, moments, _ := analyzer.Analyze(points, 100)
	if len(moments) != 2 {
		t.Fatalf("expected 2 moments after suppression, got %v", moments)
	}
	if !reflect.DeepEqual(moments, []int{25, 75}) {
		t.Fatalf("unexpected moments with suppression, got %v want [25 75]", moments)
	}
}
