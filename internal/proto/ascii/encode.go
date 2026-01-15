package ascii

func EncodeGet(key []byte) []byte {
	ln := 4 + len(key)
	b := make([]byte, ln+2)

	b[0], b[1], b[2], b[3] = 'g', 'e', 't', ' '
	copy(b[4:], key)
	b[ln], b[ln+1] = '\r', '\n'
	return b
}

func EncodeGetMulti(keys [][]byte) []byte {
	need := 4 + 2 // get + crlf
	for _, k := range keys {
		need += len(k)
	}
	need += len(keys) - 1 // spaces

	b := make([]byte, need)
	b[0], b[1], b[2], b[3] = 'g', 'e', 't', ' '

	off := 4
	for i, k := range keys {
		if i > 0 {
			b[off] = ' '
			off++
		}
		copy(b[off:], k)
		off += len(k)
	}
	b[off], b[off+1] = '\r', '\n'
	return b
}

func EncodeSet(key []byte, flags uint32, exptime uint32, value []byte) []byte {
	// "set " + key + " " + flags + " " + exptime + " " + bytes + "\r\n" + value + "\r\n"
	need := len(key) + len(value) + 41

	b := make([]byte, need)

	b[0], b[1], b[2], b[3] = 's', 'e', 't', ' '
	off := 4

	copy(b[off:], key)
	off += len(key)
	b[off] = ' '
	off++

	off += putU32(b[off:], flags)
	b[off] = ' '
	off++

	off += putU32(b[off:], exptime)
	b[off] = ' '
	off++

	off += putU32(b[off:], uint32(len(value)))
	b[off], b[off+1] = '\r', '\n'
	off += 2

	copy(b[off:], value)
	off += len(value)
	b[off], b[off+1] = '\r', '\n'
	off += 2

	return b[:off]
}

func EncodeDelete(key []byte) []byte {
	ln := 7 + len(key)
	b := make([]byte, ln+2)

	b[0], b[1], b[2], b[3], b[4], b[5], b[6] = 'd', 'e', 'l', 'e', 't', 'e', ' '
	copy(b[7:], key)
	b[ln], b[ln+1] = '\r', '\n'
	return b
}

func EncodeVersion() []byte {
	return []byte("version\r\n")
}

func putU32(dst []byte, v uint32) int {
	if v < 10 {
		dst[0] = byte('0' + v)
		return 1
	}

	var buf [10]byte
	i := len(buf)

	for {
		q := v / 10
		buf[i-1] = byte('0' + (v - q*10))
		i--
		if q == 0 {
			break
		}
		v = q
	}

	n := len(buf) - i
	copy(dst, buf[i:])
	return n
}
