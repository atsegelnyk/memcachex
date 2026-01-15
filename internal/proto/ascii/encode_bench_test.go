package ascii

import "testing"

var sink []byte

func BenchmarkEncodeSet_Small(b *testing.B) {
	key := []byte("k")
	val := []byte("v")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = EncodeSet(key, 0, 0, val)
	}
}

func BenchmarkEncodeSet_Typical(b *testing.B) {
	key := []byte("hello")
	val := []byte("world")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = EncodeSet(key, 123, 60, val)
	}
}

func BenchmarkEncodeSet_MediumValue(b *testing.B) {
	key := []byte("medium")
	val := make([]byte, 256)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = EncodeSet(key, 1, 300, val)
	}
}

func BenchmarkEncodeSet_LargeValue(b *testing.B) {
	key := []byte("large")
	val := make([]byte, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = EncodeSet(key, 42, 3600, val)
	}
}
