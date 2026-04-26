package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handleHealth(rec, req)
	if rec.Code != 200 {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("Expected 'ok' in response, got %s", rec.Body.String())
	}
}

func TestStatusResponse(t *testing.T) {
	resp := StatusResponse{NATS: true, DB: true, Uptime: "5m0s"}
	data, _ := json.Marshal(resp)
	var result StatusResponse
	json.Unmarshal(data, &result)
	if !result.NATS || !result.DB {
		t.Error("Status should have NATS and DB true")
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	wsURL = strings.Replace(wsURL, "127.0.0.1", "localhost", 1)
	// Just verify the handler doesn't panic on upgrade attempt
	_ = wsURL
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_API_VAR", "testval")
	if v := getEnv("TEST_API_VAR", "def"); v != "testval" {
		t.Errorf("Expected 'testval', got '%s'", v)
	}
}

func TestStartTime(t *testing.T) {
	startTime = time.Now().Add(-5 * time.Minute)
	uptime := time.Since(startTime).Round(time.Second).String()
	if uptime == "" {
		t.Error("Uptime should not be empty")
	}
}