# Platform API DSA — Interview Guide

## 1. Where is the HashMap?

Two places:

1. Metadata store: `map[string]string`
2. LRU cache index: `map[string]*list.Element`
3. Rate limiter client lookup: `map[string]*bucket`

Average lookup is O(1).

## 2. Why use map + linked list for LRU?

A map gives O(1) lookup but does not track recency efficiently.
A doubly-linked list tracks most/least recently used entries and supports O(1) removal/movement when we already have the element pointer.

## 3. Where is Queue used?

`internal/queue/workqueue.go` uses a buffered Go channel as a bounded FIFO queue.

Purpose:

- decouple HTTP request latency from background work
- absorb small bursts
- enforce backpressure
- cap concurrent workers

## 4. Why is the queue bounded?

To prevent uncontrolled memory growth during overload. Once full, enqueue fails and the API returns 503.

## 5. Why use a worker pool?

It bounds concurrency. Three workers means at most three jobs from this queue execute simultaneously in each API pod.

## 6. What happens after scaling to multiple replicas?

Each pod gets independent:

- cache
- rate limiter state
- metadata map
- queue

PostgreSQL is shared.

This means the application is not globally rate-limited and queued work is not durable.

## 7. Production redesign

```text
                    API Gateway
                         |
                  Distributed limiter
                         |
                   Platform API
                   /          \
              Redis           Kafka/NATS
             cache             durable jobs
                   \          /
                    PostgreSQL
```

## 8. Why not use local in-memory queue in production?

A pod restart loses queued jobs. A durable broker is better when jobs must survive crashes or support cross-replica consumption.

## 9. What about priority scheduling?

Replace the FIFO channel with a heap-backed priority queue. Enqueue/dequeue become O(log n), while peeking at the top item is O(1).

## 10. What does the Principal Engineer discuss beyond code?

- SLOs
- overload policy
- backpressure
- retry storms
- idempotency
- queue durability
- cache consistency
- rate limiter consistency
- DB connection budget
- observability
- HPA behavior
- rolling deployment behavior
- failure modes
- security
