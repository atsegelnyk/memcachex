package ring

import "testing"

func BenchmarkMPSC(b *testing.B) {
	b.ReportAllocs()

	ring := NewMPSC[[]byte](1_048_576)

	item := make([]byte, 1024)

	for i := 0; i < b.N; i++ {
		ring.Push(item)
		ring.Pop()
	}
	b.StopTimer()
}

func BenchmarkSPSC(b *testing.B) {
	b.ReportAllocs()

	ring := NewSPSC[[]byte](1_048_576)

	item := make([]byte, 1024)

	for i := 0; i < b.N; i++ {
		if !ring.CanPush() {
			continue
		}
		ring.Push(item)
		ring.Pop()
	}
	b.StopTimer()
}
