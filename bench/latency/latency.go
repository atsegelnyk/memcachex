package latency

import (
	"sync/atomic"
	"time"
)

type LatencyHist struct {
	maxUS   int64
	buckets []atomic.Int64
	count   atomic.Int64
	sumUS   atomic.Int64
	maxSeen atomic.Int64
}

func NewLatencyHist(maxUS int64) *LatencyHist {
	b := make([]atomic.Int64, maxUS+2)
	return &LatencyHist{maxUS: maxUS, buckets: b}
}

func (h *LatencyHist) Observe(d time.Duration) {
	us := d.Microseconds()
	if us < 0 {
		us = 0
	}
	h.count.Add(1)
	h.sumUS.Add(us)

	for {
		prev := h.maxSeen.Load()
		if us <= prev || h.maxSeen.CompareAndSwap(prev, us) {
			break
		}
	}

	idx := us
	if idx > h.maxUS {
		idx = h.maxUS + 1
	}
	h.buckets[idx].Add(1)
}

func (h *LatencyHist) Snapshot() (count int64, avgUS float64, maxUS int64, p50, p95, p99 int64) {
	count = h.count.Load()
	if count == 0 {
		return 0, 0, 0, 0, 0, 0
	}
	avgUS = float64(h.sumUS.Load()) / float64(count)
	maxUS = h.maxSeen.Load()

	t50 := int64(float64(count) * 0.50)
	t95 := int64(float64(count) * 0.95)
	t99 := int64(float64(count) * 0.99)
	if t50 < 1 {
		t50 = 1
	}
	if t95 < 1 {
		t95 = 1
	}
	if t99 < 1 {
		t99 = 1
	}

	var cum int64
	var got50, got95, got99 bool

	for i := int64(0); i < int64(len(h.buckets)); i++ {
		c := h.buckets[i].Load()
		if c == 0 {
			continue
		}
		cum += c

		if !got50 && cum >= t50 {
			p50 = bucketValue(i, h.maxUS)
			got50 = true
		}
		if !got95 && cum >= t95 {
			p95 = bucketValue(i, h.maxUS)
			got95 = true
		}
		if !got99 && cum >= t99 {
			p99 = bucketValue(i, h.maxUS)
			got99 = true
		}
		if got50 && got95 && got99 {
			break
		}
	}

	return
}

func bucketValue(idx, maxUS int64) int64 {
	if idx == maxUS+1 {
		return -1
	}
	return idx
}
