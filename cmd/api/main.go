package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"k8s-platform-api-minikube/internal/cache"
	"k8s-platform-api-minikube/internal/metadata"
	"k8s-platform-api-minikube/internal/queue"
	"k8s-platform-api-minikube/internal/ratelimit"
	"k8s-platform-api-minikube/internal/store"
)

type App struct {
	cache   *cache.LRU
	meta    *metadata.Store
	limiter *ratelimit.Manager
	queue   *queue.WorkQueue
	db      *store.PostgresStore
}

type createJobRequest struct {
	Name     string `json:"name"`
	Priority int    `json:"priority"`
}

func main() {
	port := env("PORT", "8080")
	dsn := env("DATABASE_URL", "postgres://platform:platform@localhost:5432/platform?sslmode=disable")

	db, err := store.NewPostgresStore(dsn)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	defer db.Close()

	app := &App{
		cache:   cache.NewLRU(100),
		meta:    metadata.NewStore(),
		limiter: ratelimit.NewManager(20, 15), // capacity=20, refill=15/sec/client
		db:      db,
	}
	app.queue = queue.NewWorkQueue(100, 3, app.processJob)
	app.queue.Start()
	defer app.queue.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.health)
	mux.HandleFunc("/readyz", app.ready)
	mux.HandleFunc("/api/v1/metadata/", app.rateLimit(app.metadataHandler))
	mux.HandleFunc("/api/v1/jobs", app.rateLimit(app.jobsHandler))
	mux.HandleFunc("/api/v1/jobs/", app.rateLimit(app.jobByIDHandler))
	mux.HandleFunc("/debug/cache", app.rateLimit(app.cacheStats))
	mux.HandleFunc("/debug/queue", app.rateLimit(app.queueStats))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           logging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("platform-api listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func (a *App) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.Header.Get("X-Client-ID")
		if clientID == "" {
			clientID = r.RemoteAddr
		}
		if !a.limiter.Allow(clientID) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": "rate limit exceeded",
			})
			return
		}
		next(w, r)
	}
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *App) metadataHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[len("/api/v1/metadata/"):]
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metadata key required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if value, ok := a.cache.Get("meta:" + key); ok {
			writeJSON(w, http.StatusOK, map[string]any{
				"key": key, "value": value, "source": "lru-cache",
			})
			return
		}
		value, ok := a.meta.Get(key)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		a.cache.Put("meta:"+key, value)
		writeJSON(w, http.StatusOK, map[string]any{
			"key": key, "value": value, "source": "metadata-map",
		})

	case http.MethodPut:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Value == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must contain value"})
			return
		}
		a.meta.Put(key, body.Value)
		a.cache.Put("meta:"+key, body.Value)
		writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": body.Value})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) jobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.Priority == 0 {
		req.Priority = 100
	}

	job, err := a.db.CreateJob(r.Context(), req.Name, req.Priority)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := a.queue.Enqueue(queue.Job{
		ID:       job.ID,
		Name:     job.Name,
		Priority: job.Priority,
	}); err != nil {
		_ = a.db.UpdateJobStatus(r.Context(), job.ID, "queue_rejected")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "work queue full"})
		return
	}

	a.cache.Put("job:"+strconv.FormatInt(job.ID, 10), job)
	writeJSON(w, http.StatusAccepted, job)
}

func (a *App) jobByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	idText := r.URL.Path[len("/api/v1/jobs/"):]
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid job id"})
		return
	}
	key := "job:" + idText

	if cached, ok := a.cache.Get(key); ok {
		writeJSON(w, http.StatusOK, map[string]any{"source": "lru-cache", "job": cached})
		return
	}

	job, err := a.db.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	a.cache.Put(key, job)
	writeJSON(w, http.StatusOK, map[string]any{"source": "postgres", "job": job})
}

func (a *App) processJob(job queue.Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = a.db.UpdateJobStatus(ctx, job.ID, "running")
	time.Sleep(400 * time.Millisecond) // simulate work
	_ = a.db.UpdateJobStatus(ctx, job.ID, "completed")

	if updated, err := a.db.GetJob(ctx, job.ID); err == nil {
		a.cache.Put("job:"+strconv.FormatInt(job.ID, 10), updated)
	}
}

func (a *App) cacheStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.cache.Stats())
}

func (a *App) queueStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.queue.Stats())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s duration=%s", r.Method, r.URL.Path, time.Since(start))
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ = fmt.Sprintf
var _ = sync.Mutex{}
