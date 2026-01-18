# memcachex

`memcachex` is an **advanced Memcached client for Go**, built for systems where latency predictability, allocation behavior, and execution control are critical.

At its core, `memcachex` uses a custom **event-loop–driven I/O engine** with async-first APIs and explicit request lifecycle management. 
The design deliberately avoids goroutine-per-request patterns, hidden buffering, and implicit scheduling decisions in favor of transparent, measurable behavior under load.

The library prioritizes:

- **Predictable latency**, especially at high concurrency
- **Near-zero allocations** on the hot path
- **Explicit control over execution and backpressure**
- **Scalable concurrency** without per-request goroutines

`memcachex` is an **advanced client** intended for performance-sensitive and infrastructure-grade workloads. It favors explicitness and control while retaining a clear and usable API.

---

## Features

- 🚀 **Async-first API** — callback-based design with no goroutine per request
- ⚡ **Event-loop–driven network engine** — explicit scheduling and predictable execution
- ♻️ **Explicit request and buffer pooling** — minimizes allocations and GC pressure
- 🧵 **Optional OS-thread pinning** — enables tighter latency control in specialized setups
- 📉 **Allocation-aware hot paths** — designed to keep steady-state allocations near zero
- 🔁 **Synchronous APIs built on the same async engine** — no duplicated code paths or behavior differences
- ⛔ **Bounded internal queues** — backpressure is applied early instead of silently buffering
- 📊 **Predictable behavior under load** — avoids latency cliffs caused by hidden buffering

---

## Installation

```bash
go get github.com/atsegelnyk/memcachex
```

---

## Creating a Client

```go
cl, err := memcachex.NewClient(
    memcachex.WithAddr("localhost:11211"),
)
if err != nil {
    panic(err)
}
```

## Client configuration

`memcachex` clients can be configured either via **functional options** or by passing a single **ClientOptions** struct.  
Both approaches configure the same internal engine and are functionally equivalent.

### Configure via functional options

```go
client, err := memcachex.NewClient(
    memcachex.WithAddr("127.0.0.1:11211"),
    memcachex.WithNumEventLoops(1),
    memcachex.WithNumEventLoopSockets(2),
    memcachex.WithRingSize(8192),
    memcachex.WithNumEnqueueRetries(2),
    memcachex.WithLockOSThread(false),
)
```

### Configure via `ClientOptions` struct

```go
opts := &memcachex.ClientOptions{
    Addr:                "127.0.0.1:11211",
    NumEventLoops:       1,
    NumEventLoopSockets: 2,
    RingSize:            8192,
    NumEnqueueRetries:   2,
    LockOSThread:        false,
}

client, err := memcachex.NewClient(
    memcachex.WithOptions(opts),
)
```

---

## Options overview

⚠️ **Note that:**

**All default values are already tuned to be optimal for the vast majority of use cases. Most applications should work well without changing any configuration unless operating under specific, benchmarked constraints (for example high RTT or extreme concurrency).**

### Address

The Memcached server address the client connects to.

* Default: `localhost:11211`

---

### Event loops

Number of independent event loops used by the client.

Each event loop:

* runs in its own goroutine
* owns its own set of sockets
* has its own request ring buffer

**Recommended values**

* Use **1 event loop** for workloads up to ~**300k RPS**
* Adding more event loops below this threshold usually adds overhead
* Increase only if you are clearly CPU-bound and have validated it via benchmarks

---

### Connections per event loop

Number of TCP connections created per event loop.

**Guidelines**

* Keep this value low
* Typical: **2–4**

All requests are pipelined internally, opening too many connections will increase contention and syscall overhead.

---

### Enqueue retries

Number of retry attempts when an event loop is temporarily busy.

Between retries, the goroutine yields via `runtime.Gosched()`.

**Recommended values**

* Optimal range: **1–3**
* `0` → fail fast under contention
* Higher values rarely help and may increase tail latency

---

### Locking OS threads

Controls whether each event loop goroutine is pinned to its OS thread using `runtime.LockOSThread`.

**Guidelines**

* Disabled by default
* Consider enabling only for controlled low-latency experiments
* Always benchmark with and without it

---

### Ring buffer size

Size of the request ring buffer used by each event loop.

**Important constraint**

* The ring size **MUST be a power of two**
* Any other value will **panic at runtime**
* Do not tune this unless you have a clear, measured need

This requirement exists due to internal index masking and fast modulo operations.

---

## Summary

* Prefer **1 event loop** unless proven otherwise
* Keep **connections low**
* Use **1–3 enqueue retries** for best balance
* Benchmark every change


---

## Sync API

### Get

```go
val, err := cl.Get([]byte("key"))
if err != nil {
    return err
}

fmt.Println(string(val.Value))
```

### GetMulti

```go
vals, err := cl.GetMulti([]byte("key1"), []byte("key2"))
if err != nil {
    return err
}

for _, val := range vals {
    fmt.Println(string(val.Value))
}
```

### Set

```go
err := cl.Set(&proto.Item{
    Key:        []byte("key"),
    Value:      []byte("value"),
    Expiration: 10,
})
```

### Version

```go
ver, err := cl.Version()
if err != nil {
    return err
}

fmt.Println(string(ver))
```

---

## Async API

Async APIs enqueue a request and invoke a callback **when the response is ready**.
No goroutine is created per request.

### Callback type

```go
type CallerCallback func(v any, e error)
```

The callback receives:

* `v any` — decoded response value (type depends on command)
* `e error` — non-nil on failure

⚠️ **The user is responsible for type-asserting `v`** to the expected response type.

This mirrors the sync API return types.

---

### Async Get

```go
err := cl.GetAsync([]byte("key"), func(v any, err error) {
    if err != nil {
        return
    }

    val := v.(*proto.Value)
    fmt.Println(string(val.Value))
})
```

### Async GetMulti

```go
err := cl.GetMultiAsync(func(v any, err error) {
    if err != nil {
        return
    }

    vals := v.([]*proto.Value)
    for _, val := range vals {
        fmt.Println(string(val.Value))
    }
	
}, []byte("key1"), []byte("key2"))
```

### Async Set

```go
err := cl.SetAsync(&proto.Item{
    Key:   []byte("key"),
    Value: []byte("value"),
}, func(_ any, err error) {
    if err != nil {
        // handle error
    }
})
```

### Async Delete

```go
err := cl.DeleteAsync([]byte("key"), func(_ any, err error) {
    if err != nil {
    // handle error
    }
})
```

### Async Version

```go
err := cl.VersionAsync(func(v any, err error) {
    if err != nil {
        return
    }

    ver := v.([]byte)
    fmt.Println(string(ver))
})
```

---

## Sync ↔ Async Return Type Mapping

| Command  | Sync return type | Async `v` type   |
|----------|------------------|------------------|
| Get      | `*proto.Value`   | `*proto.Value`   |
| GetMulti | `[]*proto.Value` | `[]*proto.Value` |
| Set      | `error`          | `nil`            |
| Delete   | `error`          | `nil`            |
| Version  | `[]byte`         | `[]byte`         |

---

## Buffer Ownership & Safety

✅ **All returned buffers are safe to use by the caller.**

* Returned `[]byte` values are **owned by the caller**
* They **do not reference internal pooled buffers**
* They remain valid after the call or callback returns
* No copying is required for safety

Internal pooling is fully encapsulated and does **not** leak buffer lifetimes into the public API.

---

## Architecture Overview

* One or more **event loops**
* Each loop owns:

    * its own sockets
    * read/write buffers
    * inflight request queues
* Requests are enqueued into the engine
* Responses are delivered via:

    * synchronous wait (`roundTrip`)
    * or async callback (`CallerCallback`)

No goroutine is spawned per request.

---

## Memory Model

* Requests are pooled internally
* Buffers are reused internally
* Public APIs return safe, caller-owned data
* Allocation avoidance is prioritized without exposing unsafe lifetimes

---


## Intended Audience

This library is for users who:

* operate at high request rates
* care about tail latency
* want explicit async control
* prefer predictable performance over convenience abstractions

---

## Status

**Experimental.**

APIs may change as performance and correctness trade-offs are refined.

---

## License

### MIT
