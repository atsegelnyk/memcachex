package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"bench/latency"

	"github.com/atsegelnyk/memcachex/proto"
)

type ClientConfig struct {
	Addr string

	// memcachex
	NumEventLoops     int
	ConnsPerEventLoop int
	LockOSThread      bool
}

type Bench struct {
	Client *Client
	Addr   string

	Workers      int
	OpsPerWorker int
	ValueLen     int

	EventLoops   int
	ConnsPerLoop int

	StaticValues bool
	Pipeline     int

	getLat *latency.LatencyHist
	setLat *latency.LatencyHist

	ops    atomic.Int64
	errors atomic.Int64
}

func main() {
	//client
	addr := flag.String("addr", "127.0.0.1:11211", "Server address")

	// memcachex client
	eventLoops := flag.Int("event-loops", 1, "Number of memcachex event loops")
	connsPerLoop := flag.Int("conns-per-loop", 2, "Connections per event loop")
	lockOSThread := flag.Bool("lock-os-thread", false, "Lock OS Thread")

	// benchmark
	workers := flag.Int("workers", 10, "Number of worker goroutines")
	opsPerWorker := flag.Int("ops", 100_000, "SET+GET operations per worker")
	valueLen := flag.Int("value-len", 256, "Value size in bytes")
	staticValues := flag.Bool("static", false, "Reuse the same value buffer")
	pipeline := flag.Int("pipeline", 1, "Number of pipelined ops, default: 1 (no pipelining)")

	flag.Parse()

	clientConfig := &ClientConfig{
		Addr:              *addr,
		NumEventLoops:     *eventLoops,
		ConnsPerEventLoop: *connsPerLoop,
		LockOSThread:      *lockOSThread,
	}
	client, err := NewClient(clientConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if *pipeline > 1 && client.Name() == "gomemcache" {
		fmt.Printf("gomemcache does not support pipelining\n")
		os.Exit(1)
	}

	fmt.Printf("=== memcache benchmark ===\n")
	fmt.Printf("go: %s | CPUs: %d\n", runtime.Version(), runtime.NumCPU())
	fmt.Printf("client: %s\n", client.Name())
	fmt.Printf("memcached: %s\n", *addr)
	fmt.Printf("workers: %d | ops/worker: %d\n", *workers, *opsPerWorker)
	fmt.Printf("value size: %d bytes | static: %t\n", *valueLen, *staticValues)
	fmt.Printf("event loops: %d | conns/loop: %d\n", *eventLoops, *connsPerLoop)
	if *pipeline > 1 {
		fmt.Printf("pipeline ops: %d\n", *pipeline)
	}
	fmt.Printf("========== start ==========\n")

	test := &Bench{
		Client:       client,
		Addr:         *addr,
		Workers:      *workers,
		OpsPerWorker: *opsPerWorker,
		ValueLen:     *valueLen,
		EventLoops:   *eventLoops,
		ConnsPerLoop: *connsPerLoop,
		StaticValues: *staticValues,
		Pipeline:     *pipeline,
	}

	test.getLat = latency.NewLatencyHist(100_000) // 100ms
	test.setLat = latency.NewLatencyHist(100_000)
	test.Run()
}

func (b *Bench) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go b.opsLogger(ctx)

	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < b.Workers; i++ {
		wg.Add(1)
		if b.Pipeline > 1 {
			go b.pipelinedWorker(i, &wg)
		} else {
			go b.syncWorker(i, &wg)
		}
	}

	wg.Wait()
	elapsed := time.Since(start)

	total := b.ops.Load()
	errs := b.errors.Load()

	fmt.Printf("=== RESULT ===\n")
	fmt.Printf("ops: %d | Errors: %d\n", total, errs)
	fmt.Printf("elapsed: %s\n", elapsed)
	fmt.Printf("throughput: %.2f ops/sec\n", float64(total)/elapsed.Seconds())

	_, _, gmax, gp50, gp95, gp99 := b.getLat.Snapshot()
	_, _, smax, sp50, sp95, sp99 := b.setLat.Snapshot()

	fmt.Printf("=== LATENCY ===\n")
	fmt.Printf("GET  p50=%s p95=%s p99=%s max=%s\n",
		fmtUS(gp50), fmtUS(gp95), fmtUS(gp99), fmtUS(gmax),
	)
	fmt.Printf("SET  p50=%s p95=%s p99=%s max=%s\n",
		fmtUS(sp50), fmtUS(sp95), fmtUS(sp99), fmtUS(smax),
	)
}

func (b *Bench) syncWorker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	rng := rand.New(rand.NewSource(int64(id) + time.Now().UnixNano()))
	staticVal := randomValue(rng, b.ValueLen)

	for i := 0; i < b.OpsPerWorker; i++ {
		key := makeKey(rng, id, i)
		val := staticVal
		if !b.StaticValues {
			val = randomValue(rng, b.ValueLen)
		}

		b.ops.Add(1)
		st := time.Now()
		err := b.Client.Set(key, val, 3600)
		b.setLat.Observe(time.Since(st))
		if err != nil {
			b.errors.Add(1)
			continue
		}

		b.ops.Add(1)
		st = time.Now()
		got, err := b.Client.Get(key)
		b.getLat.Observe(time.Since(st))
		if err != nil || got == nil || !bytes.Equal(got, val) {
			b.errors.Add(1)
		}
	}
}

func (b *Bench) pipelinedWorker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	rng := rand.New(rand.NewSource(int64(id) + time.Now().UnixNano()))
	staticVal := randomValue(rng, b.ValueLen)

	type job struct {
		key []byte
		val []byte
	}

	pipeline := make([]job, 0, b.Pipeline)

	flush := func(batch []job) {
		var wg sync.WaitGroup
		wg.Add(len(batch) * 2)

		for _, j := range batch {
			b.ops.Add(1)
			st := time.Now()
			err := b.Client.SetAsync(j.key, j.val, 3600, func(_ any, e error) {
				b.setLat.Observe(time.Since(st))
				if e != nil {
					b.errors.Add(1)
				}
				wg.Done()
			})
			if err != nil {
				b.errors.Add(1)
				wg.Done()
				continue
			}
		}

		for _, j := range batch {
			b.ops.Add(1)
			st := time.Now()
			err := b.Client.GetAsync(j.key, func(v any, e error) {
				b.getLat.Observe(time.Since(st))
				val, ok := v.(*proto.Value)
				if e != nil || !ok || !bytes.Equal(val.Value, j.val) {
					b.errors.Add(1)
				}
				wg.Done()
			})
			if err != nil {
				b.errors.Add(1)
				wg.Done()
				continue
			}
		}

		wg.Wait()
	}

	for i := 0; i < b.OpsPerWorker; i++ {
		key := makeKey(rng, id, i)
		val := staticVal
		if !b.StaticValues {
			val = randomValue(rng, b.ValueLen)
		}

		pipeline = append(pipeline, job{key: key, val: val})
		if len(pipeline) == b.Pipeline {
			flush(pipeline)
			pipeline = pipeline[:0]
		}
	}

	if len(pipeline) > 0 {
		flush(pipeline)
	}
}

func (b *Bench) opsLogger(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	var last int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cur := b.ops.Load()
			fmt.Printf("ops/sec: %d\n", cur-last)
			last = cur
		}
	}
}

func makeKey(rng *rand.Rand, wid, i int) []byte {
	return []byte(
		"key-" + strconv.Itoa(wid) + "-" + strconv.Itoa(i) + "-" + strconv.Itoa(rng.Int()),
	)
}

func randomValue(rng *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		switch rng.Intn(100) {
		case 0:
			b[i] = '\n'
		case 1:
			b[i] = '\r'
		case 2:
			b[i] = '\t'
		default:
			b[i] = byte(rng.Intn(256))
		}
	}
	return b
}

func fmtUS(us int64) string {
	if us < 0 {
		return "overflow"
	}
	if us < 1000 {
		return fmt.Sprintf("%dus", us)
	}
	return fmt.Sprintf("%.2fms", float64(us)/1000.0)
}
