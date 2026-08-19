package benchmark

import (
	"testing"
)

func TestRegressionPercent(t *testing.T) {
	res := &PerfResult{
		WarnRegression: 5,
		FailRegression: 10,
		Scenarios: []*ScenarioPerf{
			{Name: "google -> google", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 2_310_000}},
			}},
			{Name: "stdlib -> stdlib", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 2_290_000}},
			}},
		},
	}
	got := res.RegressionPercent()
	if got < 0.8 || got > 0.9 {
		t.Fatalf("expected ~0.87%% regression, got %f%%", got)
	}
	if res.Verdict() != "PASS" {
		t.Fatalf("expected PASS verdict, got %s", res.Verdict())
	}
}

func TestVerderFailAboveThreshold(t *testing.T) {
	res := &PerfResult{
		WarnRegression: 5,
		FailRegression: 10,
		Scenarios: []*ScenarioPerf{
			{Name: "google -> google", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 2_310_000}},
			}},
			{Name: "stdlib -> stdlib", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 1_900_000}},
			}},
		},
	}
	reg := res.RegressionPercent()
	if reg < 17 || reg > 18 {
		t.Fatalf("expected ~17.7%% regression, got %f%%", reg)
	}
	if res.Verdict() != "FAIL" {
		t.Fatalf("expected FAIL verdict, got %s", res.Verdict())
	}
}

func TestWarningVerdict(t *testing.T) {
	res := &PerfResult{
		WarnRegression: 5,
		FailRegression: 10,
		Scenarios: []*ScenarioPerf{
			{Name: "google -> google", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 1_000_000}},
			}},
			{Name: "stdlib -> stdlib", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 930_000}},
			}},
		},
	}
	if res.Verdict() != "WARNING" {
		t.Fatalf("expected WARNING verdict, got %s", res.Verdict())
	}
	if !res.AllPass() {
		t.Fatal("WARNING should count as pass for CI exit code")
	}
}

func TestDetectSaturation(t *testing.T) {
	res := &PerfResult{
		Scenarios: []*ScenarioPerf{
			{Name: "google -> google"},
			{Name: "stdlib -> stdlib", ConcurrencyResults: []PerfAtConcurrency{
				{Concurrency: 1, InsertRowsPerSec: Metric{Median: 120_000}},
				{Concurrency: 2, InsertRowsPerSec: Metric{Median: 240_000}},
				{Concurrency: 4, InsertRowsPerSec: Metric{Median: 470_000}},
				{Concurrency: 8, InsertRowsPerSec: Metric{Median: 900_000}},
				{Concurrency: 16, InsertRowsPerSec: Metric{Median: 1_600_000}},
				{Concurrency: 32, InsertRowsPerSec: Metric{Median: 2_200_000}},
				{Concurrency: 64, InsertRowsPerSec: Metric{Median: 2_210_000}},
			}},
		},
	}
	res.detectSaturation()
	if len(res.SaturationNotes) != 1 {
		t.Fatalf("expected 1 saturation note, got %d", len(res.SaturationNotes))
	}
	want := "concurrency=32"
	if !contains(res.SaturationNotes[0], want) {
		t.Fatalf("expected saturation around %s, got %q", want, res.SaturationNotes[0])
	}
}

func TestMetricFinalize(t *testing.T) {
	m := &Metric{}
	for _, v := range []float64{300, 100, 200} {
		m.PerIter = append(m.PerIter, v)
	}
	m.finalize()
	if m.Min != 100 || m.Median != 200 || m.Max != 300 {
		t.Fatalf("min/median/max wrong: %v %v %v", m.Min, m.Median, m.Max)
	}
}

func TestPercentileSorted(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentileSorted(sorted, 0.5); got != 5 {
		t.Fatalf("p50 = %v, want 5", got)
	}
	if got := percentileSorted(sorted, 0.95); got != 10 {
		t.Fatalf("p95 = %v, want 10", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s[:len(sub)] == sub || len(s) > len(sub) && containsSub(s, sub))
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
