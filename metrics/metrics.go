// Package metrics provides health endpoints and Prometheus metrics for the controller
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the metrics HTTP server
type Server struct {
	server *http.Server
	ready  *atomic.Bool
}

// Metrics holds all the Prometheus metrics for the application
type Metrics struct {
	SyncTotal       prometheus.Counter
	SyncErrors      prometheus.Counter
	SyncDuration    prometheus.Histogram
	DNSRecordsTotal prometheus.Gauge
	TraefikNodes    prometheus.Gauge
	LastSyncTime    prometheus.Gauge
}

// AppMetrics is the global metrics instance
var AppMetrics *Metrics

// metricsOnce
var metricsOnce sync.Once

// NewServer creates a new metrics server
func NewServer(port int) *Server {
	ready := &atomic.Bool{}
	ready.Store(false)

	// Initialize metrics only once
	metricsOnce.Do(func() {
		AppMetrics = &Metrics{
			SyncTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "nomad_traefik_controller_sync_total",
				Help: "Total number of DNS sync operations performed",
			}),
			SyncErrors: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "nomad_traefik_controller_sync_errors_total",
				Help: "Total number of DNS sync errors",
			}),
			SyncDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:    "nomad_traefik_controller_sync_duration_seconds",
				Help:    "Duration of DNS sync operations in seconds",
				Buckets: prometheus.DefBuckets,
			}),
			DNSRecordsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "nomad_traefik_controller_dns_records_total",
				Help: "Current number of DNS records managed",
			}),
			TraefikNodes: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "nomad_traefik_controller_traefik_nodes",
				Help: "Current number of healthy Traefik nodes",
			}),
			LastSyncTime: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "nomad_traefik_controller_last_sync_timestamp",
				Help: "Timestamp of the last successful sync operation",
			}),
		}

		// Register metrics with Prometheus
		prometheus.MustRegister(
			AppMetrics.SyncTotal,
			AppMetrics.SyncErrors,
			AppMetrics.SyncDuration,
			AppMetrics.DNSRecordsTotal,
			AppMetrics.TraefikNodes,
			AppMetrics.LastSyncTime,
		)
	})

	// Create HTTP mux
	mux := http.NewServeMux()
	// Health endpoint - returns 200 if the application is running
	// We do not do anything with the actual request, so we discard it for now.
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`))
	})

	// Ready endpoint - returns 200 if the application is ready to serve traffic
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if ready.Load() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ready", "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "not ready", "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`))
		}
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		server: server,
		ready:  ready,
	}
}

// Start starts the metrics server
func (s *Server) Start(ctx context.Context) error {
	log.Info("Starting metrics server", "addr", s.server.Addr)

	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Metrics server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	log.Info("Shutting down metrics server...")

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown server gracefully
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		log.Error("Metrics server shutdown error", "error", err)
		return err
	}

	log.Info("Metrics server stopped")
	return nil
}

// SetReady marks the application as ready
func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
	if ready {
		log.Info("Application marked as ready")
	} else {
		log.Info("Application marked as not ready")
	}
}

// RecordSyncStart records the start of a sync operation
func RecordSyncStart() func(error, int, int) {
	start := time.Now()
	return func(err error, dnsRecords, traefikNodes int) {
		if AppMetrics == nil {
			return // Metrics not initialized
		}

		duration := time.Since(start).Seconds()

		AppMetrics.SyncTotal.Inc()
		AppMetrics.SyncDuration.Observe(duration)
		AppMetrics.DNSRecordsTotal.Set(float64(dnsRecords))
		AppMetrics.TraefikNodes.Set(float64(traefikNodes))

		if err != nil {
			AppMetrics.SyncErrors.Inc()
		} else {
			AppMetrics.LastSyncTime.Set(float64(time.Now().Unix()))
		}
	}
}
