# Benchmarks

This document presents end-to-end benchmarks comparing **memcachex** and **gomemcache**
under identical workloads and environments.

All benchmarks perform **SET + GET** operations over TCP against a real Memcached server.

---

## Environment

### Client
- Machine type: **c4-standard-2** (2 vCPU)
- Go: **1.25.6**

### Memcached
- Machine type: **c4-standard-4** (4 vCPU)

### Common
- Value size: **256 bytes**
- Keys: random
- Values: static
- Event loops: **1**
- Connections per loop: **2**
- Network: internal GCP network (same AZ)
- Warmup: **none**

> CPU usage is reported as a percentage of available vCPUs  
> (e.g. 200% = full utilization of 2 vCPUs).

---

## Non-Pipelined Benchmarks

These runs use synchronous request/response semantics.
Latency primarily reflects network + server processing time.

### 10 workers × 100k ops per worker

| Client | Throughput | GET p50 / p99 | SET p50 / p99 | Client CPU | Memcached CPU |
|-------|------------|---------------|---------------|------------|---------------|
| gomemcache | 104k ops/s | 89μs / 180μs | 88μs / 191μs | 140% | 120% |
| memcachex | 100k ops/s | 94μs / 179μs | 95μs / 182μs | 85% | 35% |

### 100 workers × 50k ops per worker

| Client | Throughput | GET p50 / p99 | SET p50 / p99 | Client CPU | Memcached CPU |
|-------|------------|---------------|---------------|------------|---------------|
| gomemcache | 138k ops/s | 576μs / 2.91ms | 569μs / 2.67ms | 190% | 170% |
| memcachex | **460k ops/s** | **191μs / 759μs** | **190μs / 761μs** | 155% | 85% |

### 500 workers × 10k ops per worker

| Client | Throughput | GET p50 / p99 | SET p50 / p99 | Client CPU | Memcached CPU |
|-------|------------|---------------|---------------|------------|---------------|
| gomemcache | 135k ops/s | 3.46ms / 9.82ms | 3.45ms / 8.43ms | 195% | 180% |
| memcachex | **590k ops/s** | **746μs / 2.31ms** | **745μs / 2.32ms** | 185% | 100% |

### Notes
- memcachex shows similar performance at low concurrency.
- Under higher concurrency, memcachex provides significantly higher throughput
  and lower tail latency.
- CPU usage on both the client and Memcached server is substantially lower with memcachex.

---

## Pipelined Mode (memcachex)

These benchmarks evaluate aggressive request pipelining.
They are **not directly comparable** to non-pipelined runs.

Pipelined mode prioritizes throughput over per-request latency.
Reported latency includes client-side queueing time introduced by pipelining.

### Configuration
- Pipeline depth: 256–1024
- Workers: 2–10
- Event loops: 1
- Connections per loop: 2

### Results

| Workers | Ops/worker | Pipeline | Throughput | GET p50 / p99 | SET p50 / p99 | Errors |
|--------:|-----------:|---------:|-----------:|---------------|---------------|-------:|
| 2 | 5,000,000 | 1024 | 774,823 ops/s | 1.50ms / 2.75ms | 1.47ms / 2.98ms | 0 |
| 10 | 1,000,000 | 256 | 818,250 ops/s | 2.68ms / 4.42ms | 2.47ms / 4.29ms | 0 |

### Notes
- Errors represent rejected requests due to internal backpressure limits.
- Latency numbers are dominated by queueing effects, not network RTT.
- Pipelined mode is intended for throughput-oriented workloads where higher latency is acceptable.

---

## Summary

- **memcachex** matches gomemcache at low concurrency while using significantly less CPU.
- At higher concurrency, memcachex delivers **3–4× higher throughput** with
  **substantially lower tail latency**.
- In pipelined mode, memcachex can sustain **~0.8M ops/sec** on a 2 vCPU client,
  trading latency predictability for maximum throughput.
