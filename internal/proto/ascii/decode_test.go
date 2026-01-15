package ascii

import (
	"bytes"
	"github.com/atsegelnyk/memcachex/proto"
	"github.com/pkg/errors"
	"testing"
)

func TestDecodeValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		b      []byte
		want   proto.Value
		wantLn int
	}{
		{
			name:   "no_cas",
			b:      []byte("VALUE key 123 5\r\nhello\r\nEND\r\n"),
			want:   proto.Value{Key: []byte("key"), Flags: 123, CAS: 0, Value: []byte("hello")},
			wantLn: len("VALUE key 123 5\r\n") + 5 + len("\r\n"),
		},
		{
			name:   "with_cas",
			b:      []byte("VALUE k 0 3 987654321\r\nabc\r\nEND\r\n"),
			want:   proto.Value{Key: []byte("k"), Flags: 0, CAS: 987654321, Value: []byte("abc")},
			wantLn: len("VALUE k 0 3 987654321\r\n") + 3 + len("\r\n"),
		},
		{
			name: "binary_payload",
			b: func() []byte {
				payload := []byte{0x00, 0xFF, 0x0D, 0x0A, 0x7F}
				buf := make([]byte, 0, 64)
				buf = append(buf, []byte("VALUE bin 1 5\r\n")...)
				buf = append(buf, payload...)
				buf = append(buf, []byte("\r\nEND\r\n")...)
				return buf
			}(),
			want:   proto.Value{Key: []byte("bin"), Flags: 1, CAS: 0, Value: []byte{0x00, 0xFF, 0x0D, 0x0A, 0x7F}},
			wantLn: len("VALUE bin 1 5\r\n") + 5 + len("\r\n"),
		},
		{
			name:   "spaces_in_value_ok_length_driven",
			b:      []byte("VALUE s 0 11\r\nhello world\r\nEND\r\n"),
			want:   proto.Value{Key: []byte("s"), Flags: 0, CAS: 0, Value: []byte("hello world")},
			wantLn: len("VALUE s 0 11\r\n") + 11 + len("\r\n"),
		},
		{
			name:   "zero_length_value",
			b:      []byte("VALUE z 9 0\r\n\r\nEND\r\n"),
			want:   proto.Value{Key: []byte("z"), Flags: 9, CAS: 0, Value: []byte("")},
			wantLn: len("VALUE z 9 0\r\n") + 0 + len("\r\n"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ln := decodeValue(tt.b)
			if got == nil {
				t.Fatalf("decodeValue returned nil")
			}

			if !bytes.Equal(got.Key, tt.want.Key) {
				t.Fatalf("Key mismatch: got=%q want=%q", got.Key, tt.want.Key)
			}
			if got.Flags != tt.want.Flags {
				t.Fatalf("Flags mismatch: got=%d want=%d", got.Flags, tt.want.Flags)
			}
			if got.CAS != tt.want.CAS {
				t.Fatalf("CAS mismatch: got=%d want=%d", got.CAS, tt.want.CAS)
			}
			if !bytes.Equal(got.Value, tt.want.Value) {
				t.Fatalf("Value mismatch:\n got: % X\nwant: % X", got.Value, tt.want.Value)
			}
			if ln != tt.wantLn {
				t.Fatalf("Decoded length mismatch: got=%d want=%d",
					ln, tt.wantLn,
				)
			}

		})
	}
}

func TestDecodeGetResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		b       []byte
		want    proto.Value
		wantErr error
	}{
		{
			name:    "miss_end_only",
			b:       []byte("END\r\n"),
			wantErr: proto.ErrCacheMiss,
		},
		{
			name: "hit_no_cas",
			b:    []byte("VALUE key 123 5\r\nhello\r\nEND\r\n"),
			want: proto.Value{
				Key:   []byte("key"),
				Flags: 123,
				CAS:   0,
				Value: []byte("hello"),
			},
		},
		{
			name: "hit_with_cas",
			b:    []byte("VALUE k 0 3 987654321\r\nabc\r\nEND\r\n"),
			want: proto.Value{
				Key:   []byte("k"),
				Flags: 0,
				CAS:   987654321,
				Value: []byte("abc"),
			},
		},
		{
			name:    "client_error",
			b:       []byte("CLIENT_ERROR bad command line format\r\n"),
			wantErr: proto.ErrClientError,
		},
		{
			name:    "server_error",
			b:       []byte("SERVER_ERROR out of memory\r\n"),
			wantErr: proto.ErrServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeGetResponse(tt.b)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected err=%v, got nil", tt.wantErr)
				}

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected err=%v, got err=%v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got == nil {
				t.Fatalf("expected non-nil value")
			}

			if !bytes.Equal(got.Key, tt.want.Key) {
				t.Fatalf("Key mismatch: got=%q want=%q", got.Key, tt.want.Key)
			}
			if got.Flags != tt.want.Flags {
				t.Fatalf("Flags mismatch: got=%d want=%d", got.Flags, tt.want.Flags)
			}
			if got.CAS != tt.want.CAS {
				t.Fatalf("CAS mismatch: got=%d want=%d", got.CAS, tt.want.CAS)
			}
			if !bytes.Equal(got.Value, tt.want.Value) {
				t.Fatalf("Value mismatch:\n got: % X\nwant: % X", got.Value, tt.want.Value)
			}
		})
	}
}

func TestDecodeGetMultiResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		b       []byte
		want    []proto.Value
		wantErr error
	}{
		{
			name:    "miss_end_only",
			b:       []byte("END\r\n"),
			wantErr: proto.ErrCacheMiss,
		},
		{
			name: "two_values_no_cas",
			b: []byte(
				"VALUE a 1 1\r\nx\r\n" +
					"VALUE b 2 2\r\nyy\r\n" +
					"END\r\n",
			),
			want: []proto.Value{
				{Key: []byte("a"), Flags: 1, CAS: 0, Value: []byte("x")},
				{Key: []byte("b"), Flags: 2, CAS: 0, Value: []byte("yy")},
			},
		},
		{
			name: "mixed_with_cas",
			b: []byte(
				"VALUE k1 0 3 11\r\nabc\r\n" +
					"VALUE k2 7 1 22\r\nz\r\n" +
					"END\r\n",
			),
			want: []proto.Value{
				{Key: []byte("k1"), Flags: 0, CAS: 11, Value: []byte("abc")},
				{Key: []byte("k2"), Flags: 7, CAS: 22, Value: []byte("z")},
			},
		},
		{
			name:    "client_error",
			b:       []byte("CLIENT_ERROR bad command line format\r\n"),
			wantErr: proto.ErrClientError,
		},
		{
			name:    "server_error",
			b:       []byte("SERVER_ERROR out of memory\r\n"),
			wantErr: proto.ErrServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeGetMultiResponse(tt.b)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected err=%v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && err != tt.wantErr {
					t.Fatalf("expected err=%v, got err=%v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got=%d want=%d", len(got), len(tt.want))
			}

			for i := range tt.want {
				if got[i] == nil {
					t.Fatalf("got[%d] is nil", i)
				}
				if !bytes.Equal(got[i].Key, tt.want[i].Key) {
					t.Fatalf("[%d] Key mismatch: got=%q want=%q", i, got[i].Key, tt.want[i].Key)
				}
				if got[i].Flags != tt.want[i].Flags {
					t.Fatalf("[%d] Flags mismatch: got=%d want=%d", i, got[i].Flags, tt.want[i].Flags)
				}
				if got[i].CAS != tt.want[i].CAS {
					t.Fatalf("[%d] CAS mismatch: got=%d want=%d", i, got[i].CAS, tt.want[i].CAS)
				}
				if !bytes.Equal(got[i].Value, tt.want[i].Value) {
					t.Fatalf("[%d] Value mismatch:\n got: % X\nwant: % X", i, got[i].Value, tt.want[i].Value)
				}
			}
		})
	}
}
