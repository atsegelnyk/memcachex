package ascii

import (
	"github.com/atsegelnyk/memcachex/proto"
	"testing"
)

var sinkValue *proto.Value

func BenchmarkDecodeValue_NoCAS(b *testing.B) {
	buf := []byte("VALUE key 123 5\r\nhello\r\nEND\r\n")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sinkValue, _ = decodeValue(buf)
	}
}

func BenchmarkDecodeValue_WithCAS(b *testing.B) {
	buf := []byte("VALUE k 0 3 987654321\r\nabc\r\nEND\r\n")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sinkValue, _ = decodeValue(buf)
	}
}

func BenchmarkDecodeValue_BinaryPayload(b *testing.B) {
	payload := []byte{0x00, 0xFF, 0x0D, 0x0A, 0x7F}

	buf := make([]byte, 0, 64)
	buf = append(buf, []byte("VALUE bin 1 5\r\n")...)
	buf = append(buf, payload...)
	buf = append(buf, []byte("\r\nEND\r\n")...)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sinkValue, _ = decodeValue(buf)
	}
}

func BenchmarkDecodeValue_MediumValue(b *testing.B) {
	payload := make([]byte, 256)

	buf := make([]byte, 0, 512)
	buf = append(buf, []byte("VALUE med 42 256\r\n")...)
	buf = append(buf, payload...)
	buf = append(buf, []byte("\r\nEND\r\n")...)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sinkValue, _ = decodeValue(buf)
	}
}

func BenchmarkDecodeValue_LargeValue(b *testing.B) {
	payload := make([]byte, 4096)

	buf := make([]byte, 0, 4200)
	buf = append(buf, []byte("VALUE big 1 4096\r\n")...)
	buf = append(buf, payload...)
	buf = append(buf, []byte("\r\nEND\r\n")...)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sinkValue, _ = decodeValue(buf)
	}
}
