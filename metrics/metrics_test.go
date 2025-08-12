package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	server := NewServer(8080)

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "healthy"
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			response["status"], expected)
	}

	// Verify timestamp is present and valid
	if _, err := time.Parse(time.RFC3339, response["timestamp"]); err != nil {
		t.Errorf("Invalid timestamp format: %v", err)
	}
}

func TestReadyEndpointNotReady(t *testing.T) {
	server := NewServer(8081)

	req, err := http.NewRequest("GET", "/ready", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}

	expected := "not ready"
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			response["status"], expected)
	}
}

func TestReadyEndpointReady(t *testing.T) {
	server := NewServer(8082)
	server.SetReady(true)

	req, err := http.NewRequest("GET", "/ready", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "ready"
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			response["status"], expected)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	server := NewServer(8083)

	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	body := rr.Body.String()

	// Check that expected metrics are present
	expectedMetrics := []string{
		"nomad_traefik_controller_sync_total",
		"nomad_traefik_controller_sync_errors_total",
		"nomad_traefik_controller_sync_duration_seconds",
		"nomad_traefik_controller_dns_records_total",
		"nomad_traefik_controller_traefik_nodes",
		"nomad_traefik_controller_last_sync_timestamp",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric %s not found in metrics output", metric)
		}
	}
}

func TestSetReady(t *testing.T) {
	server := NewServer(8084)

	// Test setting ready to true
	server.SetReady(true)
	if !server.ready.Load() {
		t.Error("SetReady(true) did not set ready state correctly")
	}

	// Test setting ready to false
	server.SetReady(false)
	if server.ready.Load() {
		t.Error("SetReady(false) did not set ready state correctly")
	}
}

func TestRecordSyncStart(t *testing.T) {
	// Initialize metrics by creating a server (this will set up AppMetrics)
	_ = NewServer(8085)

	// Test successful sync
	recordMetrics := RecordSyncStart()
	recordMetrics(nil, 3, 2)

	// Verify that AppMetrics is initialized and function doesn't panic
	if AppMetrics == nil {
		t.Error("AppMetrics was not initialized")
	}
}

func TestRecordSyncStartWithError(t *testing.T) {
	// Initialize metrics by creating a server
	_ = NewServer(8086)

	// Test failed sync
	recordMetrics := RecordSyncStart()
	recordMetrics(fmt.Errorf("test error"), 0, 0)

	// Verify that AppMetrics is initialized and function doesn't panic
	if AppMetrics == nil {
		t.Error("AppMetrics was not initialized")
	}
}

func TestServerStartStop(t *testing.T) {
	server := NewServer(0) // Use port 0 to get a random available port

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for server to stop
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("Server returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

func TestNewServerInitializesMetrics(t *testing.T) {
	server := NewServer(8087)

	if AppMetrics == nil {
		t.Error("AppMetrics was not initialized")
	}

	if AppMetrics.SyncTotal == nil {
		t.Error("SyncTotal metric was not initialized")
	}

	if AppMetrics.SyncErrors == nil {
		t.Error("SyncErrors metric was not initialized")
	}

	if AppMetrics.SyncDuration == nil {
		t.Error("SyncDuration metric was not initialized")
	}

	if AppMetrics.DNSRecordsTotal == nil {
		t.Error("DNSRecordsTotal metric was not initialized")
	}

	if AppMetrics.TraefikNodes == nil {
		t.Error("TraefikNodes metric was not initialized")
	}

	if AppMetrics.LastSyncTime == nil {
		t.Error("LastSyncTime metric was not initialized")
	}

	// Verify server is properly configured
	if server.server == nil {
		t.Error("HTTP server was not initialized")
	}

	if server.ready == nil {
		t.Error("Ready atomic bool was not initialized")
	}
}
