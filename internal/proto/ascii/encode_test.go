package ascii

import (
	"bytes"
	"testing"
)

func TestEncodeSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     []byte
		flags   uint32
		exptime uint32
		value   []byte
		want    []byte
	}{
		{
			name:    "basic",
			key:     []byte("k"),
			flags:   0,
			exptime: 0,
			value:   []byte("v"),
			want:    []byte("set k 0 0 1\r\nv\r\n"),
		},
		{
			name:    "typical",
			key:     []byte("hello"),
			flags:   123,
			exptime: 60,
			value:   []byte("world"),
			want:    []byte("set hello 123 60 5\r\nworld\r\n"),
		},
		{
			name:    "empty_value",
			key:     []byte("empty"),
			flags:   42,
			exptime: 0,
			value:   []byte(""),
			want:    []byte("set empty 42 0 0\r\n\r\n"),
		},
		{
			name:    "max_u32_numbers",
			key:     []byte("k"),
			flags:   ^uint32(0),
			exptime: ^uint32(0),
			value:   []byte("abc"),
			want:    []byte("set k 4294967295 4294967295 3\r\nabc\r\n"),
		},
		{
			name:    "binary_value",
			key:     []byte("bin"),
			flags:   1,
			exptime: 2,
			value:   []byte{0x00, 0xFF, 0x0D, 0x0A, 0x7F},
			want:    append([]byte("set bin 1 2 5\r\n"), []byte{0x00, 0xFF, 0x0D, 0x0A, 0x7F, '\r', '\n'}...),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := EncodeSet(tt.key, tt.flags, tt.exptime, tt.value)

			if !bytes.Equal(got, tt.want) {
				t.Fatalf("EncodeSet() mismatch\n got: %q\nwant: %q\n got_hex: % X\nwant_hex: % X",
					got, tt.want, got, tt.want,
				)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("EncodeSet() length mismatch: got=%d want=%d", len(got), len(tt.want))
			}
		})
	}
}
