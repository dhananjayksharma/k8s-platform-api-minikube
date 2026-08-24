# Platform API — Go + Minikube DSA POC

A local interview/demo project that maps practical DSA concepts to a Platform API.

## Architecture

```text
                         Client / curl
                              |
                              v
                     Token Bucket Limiter
                              |
                              v
                     ┌─────────────────┐
                     │ Go Platform API │
                     └────────┬────────┘
                              |
          ┌───────────────────┼────────────────────┐
          │                   │                    │
          v                   v                    v
      LRU Cache          Metadata Map        Bounded Queue
   map + linked list      map+RWMutex       buffered channel
          │                                        |
          │                                  3 worker goroutines
          │                                        |
          └──────────────────┬─────────────────────┘
                             v
                         PostgreSQL
```

## DSA demonstrated

| Concept | Implementation | Why it matters |
|---|---|---|
| HashMap | `metadata.Store` | O(1)-average metadata lookup |
| LRU Cache | map + doubly-linked list | O(1) get/put and bounded memory |
| Queue | buffered Go channel | decouples request path from workers |
| Bounded queue | capacity 100 | backpressure; avoids unlimited memory growth |
| Worker pool | 3 goroutines | bounded concurrency |
| Token Bucket | per-client bucket map | API rate limiting |
| Big-O | documented per component | capacity/performance reasoning |

## 1. Prerequisites

Install:

- Go
- Docker
- Minikube
- kubectl

Check:

```bash
go version
docker version
minikube version
kubectl version --client
```

## 2. Start Minikube

```bash
minikube start --cpus=4 --memory=6144

minikube start --driver=docker --container-runtime=docker --gpus=all --cpus=6 --memory=6144 --disk-size=40g --profile=gpu
-cpu-lab --nodes=3

```

Check:

```bash
kubectl get nodes
```

Expected:

```text
NAME       STATUS   ROLES           AGE   VERSION
minikube   Ready    control-plane   ...   ...
```

## 3. Download Go dependencies

The first run needs internet access to download the PostgreSQL Go driver.


```bash
go mod tidy
go test ./...
```

This generates `go.sum`.

## 4. Build container inside Minikube

```bash
minikube -p gpu-cpu-lab image build -t k8s-platform-api-minikube:local .
```

Verify:

```bash
minikube  -p gpu-cpu-lab image ls | grep platform-api
```

## 5. Deploy

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/postgres.yaml

kubectl rollout status deployment/postgres \
  -n platform-demo \
  --timeout=120s

kubectl apply -f k8s/api.yaml

kubectl rollout status deployment/platform-api \
  -n platform-demo \
  --timeout=120s
```

Or:

```bash
make deploy
```

## 6. Verify Kubernetes resources

```bash
kubectl get all -n platform-demo
```

You should see:

- 1 PostgreSQL pod
- 2 Platform API pods
- `postgres` Service
- `platform-api` Service

Inspect logs:

```bash
kubectl logs -n platform-demo \
  -l app=platform-api \
  --tail=100
```

## 7. Access from localhost

Recommended:

```bash
kubectl port-forward \
  -n platform-demo \
  svc/postgres \
  8080:80
```

Then:

```bash
curl localhost:8080/healthz
```

Expected:

```json
{"status":"ok"}
```

Readiness:

```bash
curl localhost:8080/readyz
```

Expected:

```json
{"status":"ready"}
```

---

# API examples

## Metadata HashMap

Read built-in value:

```bash
curl -H 'X-Client-ID: demo' \
  localhost:8080/api/v1/metadata/cluster
```

Example:

```json
{
  "key":"cluster",
  "source":"metadata-map",
  "value":"minikube"
}
```

Call again:

```bash
curl -H 'X-Client-ID: demo' \
  localhost:8080/api/v1/metadata/cluster
```

Now the source can become:

```text
lru-cache
```

Update metadata:

```bash
curl -X PUT \
  -H 'Content-Type: application/json' \
  -H 'X-Client-ID: demo' \
  localhost:8080/api/v1/metadata/team \
  -d '{"value":"platform-engineering"}'
```

Read:

```bash
curl -H 'X-Client-ID: demo' \
  localhost:8080/api/v1/metadata/team
```

---

# Queue + Worker Pool + PostgreSQL

Create a job:

```bash
curl -X POST \
  -H 'Content-Type: application/json' \
  -H 'X-Client-ID: demo' \
  localhost:8080/api/v1/jobs \
  -d '{"name":"reconcile-cluster","priority":10}'
```

Example:

```json
{
  "id":1,
  "name":"reconcile-cluster",
  "priority":10,
  "status":"queued"
}
```

Fetch it:

```bash
curl -H 'X-Client-ID: demo' \
  localhost:8080/api/v1/jobs/1
```

After worker processing:

```json
{
  "source":"lru-cache",
  "job":{
    "id":1,
    "name":"reconcile-cluster",
    "priority":10,
    "status":"completed"
  }
}
```

Inspect queue:

```bash
curl -H 'X-Client-ID: demo' \
  localhost:8080/debug/queue
```

Example:

```json
{
  "capacity":100,
  "depth":0,
  "workers":3,
  "enqueued":1,
  "processed":1,
  "rejected":0
}
```

Inspect cache:

```bash
curl -H 'X-Client-ID: demo' \
  localhost:8080/debug/cache
```

---

# Rate limiter test

The POC uses:

```text
Capacity: 10 tokens
Refill:   5 tokens/second/client
```

Generate a burst:

```bash
for i in $(seq 1 25); do
  curl -s \
    -o /dev/null \
    -w "%{http_code}\n" \
    -H 'X-Client-ID: burst-test' \
    localhost:8080/api/v1/metadata/cluster
done
```

You should observe both:

```text
200
429
```

429 means:

```text
Too Many Requests
```

---

# PostgreSQL inspection

Get the PostgreSQL pod:

```bash
kubectl get pods -n platform-demo -l app=postgres
```

Open psql:

```bash
kubectl exec -it \
  -n platform-demo \
  deployment/postgres \
  -- psql -U platform -d platform
```

Then:

```sql
SELECT id, name, priority, status, created_at, updated_at
FROM jobs
ORDER BY id;
```

Exit:

```text
\q
```

---

# Scale the API

```bash
kubectl scale deployment/platform-api \
  -n platform-demo \
  --replicas=5
```

Verify:

```bash
kubectl get pods -n platform-demo -l app=platform-api
```

## Important distributed-systems observation

The current POC deliberately keeps these components in-process:

- LRU cache
- rate-limit buckets
- metadata map
- work queue

With 5 replicas, each pod has independent state.

```text
                  Service
                /    |    \
               v     v     v
            API-1  API-2  API-3
              |      |      |
           Cache1 Cache2 Cache3
           Rate1  Rate2  Rate3
           Queue1 Queue2 Queue3
```

This is useful for an interview discussion.

For production, possible changes are:

```text
Local LRU           -> Redis / distributed cache
Local rate limiter  -> Redis + atomic Lua / gateway limiter
Local queue         -> Kafka / NATS / SQS / RabbitMQ
Local metadata      -> DB / Kubernetes API / distributed KV
```

---

# Principal Engineer interview discussion

## Why a bounded queue?

An unlimited queue can create:

```text
Traffic spike
    ↓
Queue grows
    ↓
Memory increases
    ↓
GC pressure
    ↓
Latency increases
    ↓
OOMKilled
```

A bounded queue forces a decision:

```text
queue has room  -> accept
queue full      -> reject / retry / shed load
```

This POC returns HTTP 503 when the queue cannot accept new work.

## Why an LRU cache?

A normal map can grow without bound.

LRU adds eviction:

```text
Most Recently Used                       Least Recently Used
        |                                          |
        v                                          v
HEAD <-> item-D <-> item-A <-> item-C <-> TAIL
                                              |
                                              v
                                            evict
```

`map[key]*list.Element` provides lookup.

`container/list` provides O(1) movement/removal once the element is known.

Typical:

```text
Get: O(1)
Put: O(1)
Evict: O(1)
```

## Why RWMutex for metadata?

Many platform workloads are read-heavy:

```text
many readers
     |
     v
 sync.RWMutex
     |
 few writers
```

`RLock()` allows concurrent readers.

## Why worker pools?

Without bounded concurrency:

```text
1 request -> 1 goroutine
1,000,000 requests -> potentially huge goroutine growth
```

With a worker pool:

```text
requests
   |
 bounded queue
   |
+--+--+
|  |  |
W1 W2 W3
```

Concurrency is controlled.

## Why database connection-pool limits?

The code uses:

```go
db.SetMaxOpenConns(20)
db.SetMaxIdleConns(5)
```

If five API replicas exist:

```text
5 replicas × 20 max connections
= potentially 100 open connections
```

At Principal level you should calculate the database connection budget across all replicas.

---

# Important POC limitation: Priority

The API accepts `priority`, but this first POC intentionally uses a FIFO bounded queue.

That creates a good follow-up exercise:

```text
Current:
FIFO Queue

Next:
container/heap
    ↓
Priority Queue
    ↓
highest priority job first
```

Implementing this extension demonstrates:

- heap
- priority queue
- synchronization
- starvation considerations
- fairness
- scheduling policy

---

# Cleanup

```bash
kubectl delete namespace platform-demo
```

Or:

```bash
make clean
```

Stop Minikube:

```bash
minikube stop
```

Delete the cluster if no longer needed:

```bash
minikube delete
```
