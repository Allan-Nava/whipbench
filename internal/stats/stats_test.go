package stats

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestSummariseNearestRank(t *testing.T) {
	vs := make([]float64, 100)
	for i := range vs {
		vs[i] = float64(100 - i) // 100..1, unsorted on purpose
	}
	s := Summarise(vs)
	if s.N != 100 || s.Min != 1 || s.Max != 100 || s.P50 != 50 || s.P95 != 95 || s.P99 != 99 || s.Mean != 50.5 {
		t.Fatalf("summary = %+v", s)
	}
	if vs[0] != 100 {
		t.Fatal("Summarise must not sort its input in place")
	}
	// Three values: p50 is the 2nd, p95 and p99 the 3rd.
	s = Summarise([]float64{30, 10, 20})
	if s.P50 != 20 || s.P95 != 30 || s.P99 != 30 {
		t.Fatalf("summary of three = %+v", s)
	}
	if (Summarise(nil) != Summary{}) {
		t.Fatal("empty input must give the zero summary")
	}
}

func TestHistogramWithinHalfAPercent(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	h := NewHistogram()
	vs := make([]float64, 20000)
	for i := range vs {
		vs[i] = 2 + r.ExpFloat64()*15 // ms, a long right tail like real delay
		h.Add(vs[i])
	}
	exact := Summarise(vs)
	got := h.Summary()
	for _, c := range []struct {
		name      string
		got, want float64
	}{{"p50", got.P50, exact.P50}, {"p95", got.P95, exact.P95}, {"p99", got.P99, exact.P99}} {
		if rel := math.Abs(c.got-c.want) / c.want; rel > 0.006 {
			t.Errorf("%s = %.4f, exact %.4f (%.2f%% off)", c.name, c.got, c.want, rel*100)
		}
	}
	if got.Min != exact.Min || got.Max != exact.Max || math.Abs(got.Mean-exact.Mean) > 1e-9 {
		t.Errorf("min/max/mean must be exact: %+v vs %+v", got, exact)
	}
}

func TestHistogramMergeIsExact(t *testing.T) {
	a, b, all := NewHistogram(), NewHistogram(), NewHistogram()
	for i := 1; i <= 1000; i++ {
		v := float64(i) / 10
		if i%3 == 0 {
			a.Add(v)
		} else {
			b.Add(v)
		}
		all.Add(v)
	}
	a.Merge(b)
	if a.Summary() != all.Summary() {
		t.Fatalf("merged %+v != built %+v", a.Summary(), all.Summary())
	}
}

func TestHistogramExtremes(t *testing.T) {
	h := NewHistogram()
	h.Add(0)
	h.Add(1e9) // far above the top bucket
	if h.Quantile(50) != 0 || h.Quantile(100) != 1e9 {
		t.Fatalf("clamping: p50=%v p100=%v", h.Quantile(50), h.Quantile(100))
	}
}
