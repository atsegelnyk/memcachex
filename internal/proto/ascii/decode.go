package ascii

import (
	"bytes"
	"github.com/atsegelnyk/memcachex/proto"
)

var (
	ValueMarker = []byte("VALUE")
	StatMarker  = []byte("STAT")
	ItemMarker  = []byte("ITEM")

	ErrMarker       = []byte("ERROR ")
	ClientErrMarker = []byte("CLIENT_ERROR ")
	ServerErrMarker = []byte("SERVER_ERROR ")

	EndMarker = []byte("END\r\n")
)

var (
	NotStoredResp = []byte("NOT_STORED\r\n")
	NotFoundResp  = []byte("NOT_FOUND\r\n")
)

func DecodeResponseLen(resp []byte) int {
	iCRLF := fastIndexCRLF(resp)
	if iCRLF == -1 {
		return 0
	}

	offset := iCRLF + 2

	header := resp[:iCRLF+2]
	if !bytes.HasPrefix(header, ValueMarker) {
		if bytes.HasPrefix(header, StatMarker) || bytes.HasPrefix(header, ItemMarker) {
			return decodeStatsLen(resp, offset)
		}
		return offset
	}

	for {
		bodyLen := decodeResponseBodyLen(header, 4)
		offset += bodyLen + 2
		if offset > len(resp) {
			return 0
		}

		nextICRLF := fastIndexCRLF(resp[offset:])
		if nextICRLF == -1 {
			return 0
		}

		header = resp[offset : offset+nextICRLF+2]
		offset += nextICRLF + 2
		if bytes.Equal(header, EndMarker) {
			break
		}

		if !bytes.HasPrefix(header, ValueMarker) {
			return 0
		}
	}

	return offset
}

func fastIndexCRLF(buf []byte) int {
	for i := 0; i < len(buf)-1; i++ {
		if buf[i] == '\r' && buf[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func decodeStatsLen(b []byte, offset int) int {
	for {
		nextICRLF := fastIndexCRLF(b[offset:])
		if nextICRLF == -1 {
			return 0
		}

		if bytes.Equal(b[offset:offset+nextICRLF+2], EndMarker) {
			offset += nextICRLF + 2
			break
		}

		offset += nextICRLF + 2
	}
	return offset
}

func decodeResponseBodyLen(header []byte, pos int) int {
	parts := 0

	wasPart := false
	var x uint
	for i := 0; i < len(header); i++ {
		if header[i] == ' ' {
			if !wasPart {
				continue
			}
			parts++
			wasPart = false
			if parts == pos {
				return int(x)
			}
			x = 0
			continue
		}

		wasPart = true
		d := header[i] - '0'
		if d <= 9 {
			if parts < 2 {
				continue
			}

			x *= 10
			x += uint(d)

			if i == len(header)-1 && parts+1 == pos {
				return int(x)
			}
			continue
		}
		if (header[i] == '\r' || header[i] == '\n') && parts+1 == pos {
			return int(x)
		}

	}
	return -1
}

func DecodeGetResponse(b []byte) (*proto.Value, error) {
	err := responseErrors(b)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(b, EndMarker) {
		return nil, proto.ErrCacheMiss
	}

	val, _ := decodeValue(b)
	return val, nil
}

func DecodeGetMultiResponse(b []byte) ([]*proto.Value, error) {
	err := responseErrors(b)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(b, EndMarker) {
		return nil, proto.ErrCacheMiss
	}

	var vals []*proto.Value
	for {
		val, consumed := decodeValue(b)
		vals = append(vals, val)

		b = b[consumed:]

		if bytes.Equal(b, EndMarker) {
			break
		}
	}

	return vals, nil
}

func DecodeSetResponse(b []byte) error {
	err := responseErrors(b)
	if err != nil {
		return err
	}

	if bytes.Equal(b, NotStoredResp) {
		return proto.ErrNotStored
	}
	return nil
}

func DecodeDeleteResponse(b []byte) error {
	err := responseErrors(b)
	if err != nil {
		return err
	}

	if bytes.Equal(b, NotFoundResp) {
		return proto.ErrNotFound
	}
	return nil
}

func DecodeVersionResponse(b []byte) ([]byte, error) {
	err := responseErrors(b)
	if err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(b, []byte("\r\n")), nil
}

func responseErrors(b []byte) error {
	if bytes.HasPrefix(b, ErrMarker) {
		return proto.ErrMemcachedError
	}

	if bytes.HasPrefix(b, ClientErrMarker) {
		return proto.ErrClientError
	}

	if bytes.HasPrefix(b, ServerErrMarker) {
		return proto.ErrServerError
	}

	return nil
}

func decodeValue(b []byte) (*proto.Value, int) {
	icrlf := fastIndexCRLF(b)
	header := b[6:icrlf]

	off := 0

	sp := bytes.IndexByte(header, ' ')
	key := header[:sp]
	off = sp + 1

	flags, n := parseU32(header[off:])
	off += n + 1

	vlen32, n := parseU32(header[off:])
	off += n

	var cas uint64
	if off < len(header) && header[off] == ' ' {
		off++
		cas, _ = parseU64(header[off:])
	}

	dataStart := icrlf + 2
	vlen := int(vlen32)

	val := make([]byte, vlen)
	copy(val, b[dataStart:dataStart+vlen])

	return &proto.Value{Key: key, Flags: flags, CAS: cas, Value: val}, dataStart + vlen + 2
}

func parseU32(s []byte) (uint32, int) {
	var v uint32
	i := 0
	for i < len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + uint32(c-'0')
		i++
	}
	return v, i
}

func parseU64(s []byte) (uint64, int) {
	var v uint64
	i := 0
	for i < len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + uint64(c-'0')
		i++
	}
	return v, i
}
