// Package stats holds the two summaries whipbench reports: exact percentiles over a
// small set of values (one join time per viewer), and a mergeable log-bucketed
// histogram for the per-packet one-way delays, which are too many to keep.
package stats

import (
	"math"
	"sort"
)

// Summary is what a report prints for one quantity. Percentiles use the
// nearest-rank method: p is the smallest value with at least p% of the values at
// or below it, so every reported percentile is a value that was observed (exact
// summaries) or a bucket that held one (histograms).
type Summary struct {
	N    int     `json:"n"`
	Min  float64 `json:"min"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Max  float64 `json:"max"`
}

// Rank is the 1-based nearest rank of percentile p (0 < p <= 100) among n values.
func Rank(p float64, n int) int {
	r := int(math.Ceil(p / 100 * float64(n)))
	if r < 1 {
		r = 1
	}
	if r > n {
		r = n
	}
	return r
}

// Summarise returns the exact summary of vs; it does not modify vs.
func Summarise(vs []float64) Summary {
	if len(vs) == 0 {
		return Summary{}
	}
	s := append([]float64(nil), vs...)
	sort.Float64s(s)
	sum := 0.0
	for _, v := range s {
		sum += v
	}
	at := func(p float64) float64 { return s[Rank(p, len(s))-1] }
	return Summary{
		N: len(s), Min: s[0], Max: s[len(s)-1], Mean: sum / float64(len(s)),
		P50: at(50), P95: at(95), P99: at(99),
	}
}

// Histogram counts positive durations, in milliseconds, in logarithmic buckets 1%
// wide: bucket i holds [base·g^i, base·g^(i+1)) with g = 1.01 and base = 1 µs. A
// percentile read from it is the bucket's geometric midpoint, so it is within 0.5%
// of a value that was observed; min, max and mean are exact. Values below 1 µs land
// in bucket 0; values above the top (60 s) land in the last.
//
// The zero value is not usable; call NewHistogram. Not safe for concurrent use.
type Histogram struct {
	counts   []uint64
	n        uint64
	sum      float64
	min, max float64
}

const (
	histBase   = 0.001 // ms, i.e. 1 µs
	histGrowth = 1.01
	histTopMs  = 60000
)

var histBuckets = int(math.Ceil(math.Log(histTopMs/histBase)/math.Log(histGrowth))) + 1

// NewHistogram returns an empty histogram.
func NewHistogram() *Histogram {
	return &Histogram{counts: make([]uint64, histBuckets), min: math.Inf(1), max: math.Inf(-1)}
}

func bucket(ms float64) int {
	if ms <= histBase {
		return 0
	}
	i := int(math.Log(ms/histBase) / math.Log(histGrowth))
	if i >= histBuckets {
		return histBuckets - 1
	}
	return i
}

func midpoint(i int) float64 {
	return histBase * math.Pow(histGrowth, float64(i)+0.5)
}

// Add records one value in milliseconds.
func (h *Histogram) Add(ms float64) {
	h.counts[bucket(ms)]++
	h.n++
	h.sum += ms
	h.min = math.Min(h.min, ms)
	h.max = math.Max(h.max, ms)
}

// Count is the number of values recorded.
func (h *Histogram) Count() uint64 { return h.n }

// Merge adds every value of o to h. Merging is exact: the merged histogram is the
// one that would have been built from both sets of values.
func (h *Histogram) Merge(o *Histogram) {
	if o == nil || o.n == 0 {
		return
	}
	for i, c := range o.counts {
		h.counts[i] += c
	}
	h.n += o.n
	h.sum += o.sum
	h.min = math.Min(h.min, o.min)
	h.max = math.Max(h.max, o.max)
}

// Quantile returns the nearest-rank percentile p (0 < p <= 100). The lowest and
// highest ranks return the exact min and max, and every other answer is clamped to
// them, so a percentile never lies outside what was observed.
func (h *Histogram) Quantile(p float64) float64 {
	if h.n == 0 {
		return 0
	}
	rank := uint64(Rank(p, int(min(h.n, math.MaxInt)))) //nolint:gosec // bounded above
	// The first and last ranks are known exactly; only the ones between are bucketed.
	if rank == 1 {
		return h.min
	}
	if rank == h.n {
		return h.max
	}
	var seen uint64
	for i, c := range h.counts {
		seen += c
		if seen >= rank {
			return math.Max(h.min, math.Min(h.max, midpoint(i)))
		}
	}
	return h.max
}

// Summary returns the histogram's summary.
func (h *Histogram) Summary() Summary {
	if h.n == 0 {
		return Summary{}
	}
	return Summary{
		N: int(min(h.n, math.MaxInt)), Min: h.min, Max: h.max, Mean: h.sum / float64(h.n), //nolint:gosec // bounded
		P50: h.Quantile(50), P95: h.Quantile(95), P99: h.Quantile(99),
	}
}
