# memcachex

**memcachex** is a high-performance Memcached client for Go, built around an **event-loop I/O engine**, **async-first APIs**, and **explicit request management**.

The library prioritizes:

* predictable latency
* almost zero allocations
* explicit control over execution
* avoiding goroutine-per-request designs

This is a **low-level client** intended for performance-critical use cases.

---

## Features

* 🚀 Async-first API (callbacks, no goroutine per request)
* ⚡ Event-loop based network engine
* ♻️ Request and buffer pooling internally
* 🧵 Optional OS-thread pinning
* 📉 Designed for minimal allocations on hot paths
* 🔁 Sync APIs built on top of the same async engine

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
    memcachex.WithNumEventLoops(1),
)
if err != nil {
    panic(err)
}
```

### Options

```go
memcachex.WithAddr("127.0.0.1:11211")
memcachex.WithNumEventLoops(1)
memcachex.WithLockOSThread(true)
```

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

| Command | Sync return type | Async `v` type |
| ------- | ---------------- | -------------- |
| Get     | `*proto.Value`   | `*proto.Value` |
| Set     | `error`          | `nil`          |
| Version | `[]byte`         | `[]byte`       |

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
