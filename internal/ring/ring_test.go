package ring

import "testing"

func BenchmarkRing(b *testing.B) {
	b.ReportAllocs()

	ring := NewMPSC[[]byte](1024)

	item := make([]byte, 1024)

	for i := 0; i < b.N; i++ {
		ring.Push(item)
		ring.Pop()
	}
	b.StopTimer()
}
