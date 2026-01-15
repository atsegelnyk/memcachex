package proto

type Value struct {
	Key   []byte
	Flags uint32
	CAS   uint64
	Value []byte
}

type GetMultiResp struct {
	Values []Value
}
