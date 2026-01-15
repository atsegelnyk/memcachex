package proto

type Stats struct {
	PID      uint64
	Version  string
	Libevent string

	UptimeSeconds uint64
	TimeUnix      uint64

	CurrConnections     uint64
	TotalConnections    uint64
	RejectedConnections uint64

	CurrItems  uint64
	TotalItems uint64
	Evictions  uint64
	Reclaimed  uint64

	Bytes         uint64
	LimitMaxbytes uint64

	CmdGet   uint64
	CmdSet   uint64
	CmdFlush uint64
	CmdTouch uint64

	GetHits   uint64
	GetMisses uint64

	DeleteHits   uint64
	DeleteMisses uint64

	IncrHits   uint64
	IncrMisses uint64
	DecrHits   uint64
	DecrMisses uint64

	CasHits   uint64
	CasMisses uint64
	CasBadval uint64

	BytesRead    uint64
	BytesWritten uint64

	Threads uint64

	Raw map[string]string
}

type ItemsStats struct {
	Slabs map[uint32]ItemsSlabStats
}

type ItemsSlabStats struct {
	Number         uint64
	Age            uint64
	Evicted        uint64
	EvictedNonZero uint64
	Reclaimed      uint64
	Moved          uint64
	OutOfMemory    uint64
	Raw            map[string]uint64
}

type StatsResp struct {
	KVs []StatKV
}

type StatKV struct {
	Name  []byte
	Value []byte
}
