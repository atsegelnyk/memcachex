package proto

type Item struct {
	Key        []byte
	Value      []byte
	Flags      uint32
	Expiration uint32
	CAS        uint64
}
