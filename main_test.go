package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewProxyHandler(t *testing.T) {
	tests := []struct {
		name    string
		envURL  string
		wantURL string
	}{
		{
			name:    "default URL when env not set",
			envURL:  "",
			wantURL: "http://flaresolverr:8191/v1",
		},
		{
			name:    "custom URL from environment",
			envURL:  "http://custom:9999/api",
			wantURL: "http://custom:9999/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envURL != "" {
				os.Setenv("FLARESOLVERR_URL", tt.envURL)
				defer os.Unsetenv("FLARESOLVERR_URL")
			}

			handler := NewProxyHandler()
			if handler.flareSolverrURL != tt.wantURL {
				t.Errorf("NewProxyHandler() URL = %v, want %v", handler.flareSolverrURL, tt.wantURL)
			}
		})
	}
}

func TestProxyHandler_ServeHTTP(t *testing.T) {
	// Create a mock FlareSolverr server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request is correct
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Decode the request to verify it's correct
		var req FlareSolverrRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Check for special test cases based on URL
		if strings.Contains(req.URL, "error-test") {
			// Return an error response
			response := FlareSolverrResponse{
				Status:  "error",
				Message: "Test error message",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Return a successful response
		response := FlareSolverrResponse{
			Status: "ok",
		}
		response.Solution.Response = "<html><body>Test HTML Response</body></html>"
		response.Solution.Status = 200
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create handler with mock server URL
	os.Setenv("FLARESOLVERR_URL", mockServer.URL)
	defer os.Unsetenv("FLARESOLVERR_URL")
	handler := NewProxyHandler()

	tests := []struct {
		name           string
		method         string
		url            string
		wantStatus     int
		wantContent    string
		wantError      bool
	}{
		{
			name:        "successful GET request",
			method:      "GET",
			url:         "http://example.com",
			wantStatus:  http.StatusOK,
			wantContent: "<html><body>Test HTML Response</body></html>",
			wantError:   false,
		},
		{
			name:        "CONNECT request rejected",
			method:      "CONNECT",
			url:         "example.com:443",
			wantStatus:  http.StatusMethodNotAllowed,
			wantContent: "CONNECT method is not supported",
			wantError:   true,
		},
		{
			name:        "unsupported method POST",
			method:      "POST",
			url:         "http://example.com",
			wantStatus:  http.StatusMethodNotAllowed,
			wantContent: "Method not allowed",
			wantError:   true,
		},
		{
			name:        "FlareSolverr error response",
			method:      "GET",
			url:         "http://error-test.com",
			wantStatus:  http.StatusInternalServerError,
			wantContent: "Test error message",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest(tt.method, tt.url, nil)
			if tt.method == "CONNECT" {
				req.Host = "example.com:443"
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Handle request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.wantStatus {
				t.Errorf("ServeHTTP() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			// Check response content
			responseBody := rr.Body.String()
			if !strings.Contains(responseBody, tt.wantContent) {
				t.Errorf("ServeHTTP() body = %v, want to contain %v", responseBody, tt.wantContent)
			}

			// Check Content-Type header
			if tt.wantError {
				if tt.method != "POST" && !strings.Contains(rr.Header().Get("Content-Type"), "text/plain") &&
					!strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
					// Error responses should be either text/plain or application/json
					t.Errorf("ServeHTTP() error Content-Type = %v", rr.Header().Get("Content-Type"))
				}
			} else if rr.Header().Get("Content-Type") != "text/html; charset=utf-8" {
				t.Errorf("ServeHTTP() Content-Type = %v, want text/html; charset=utf-8", rr.Header().Get("Content-Type"))
			}
		})
	}
}

func TestProxyHandler_ConnectionError(t *testing.T) {
	// Create handler with invalid FlareSolverr URL
	os.Setenv("FLARESOLVERR_URL", "http://invalid-host-that-does-not-exist:12345/v1")
	defer os.Unsetenv("FLARESOLVERR_URL")
	handler := NewProxyHandler()

	// Create request
	req := httptest.NewRequest("GET", "http://example.com", nil)
	rr := httptest.NewRecorder()

	// Handle request
	handler.ServeHTTP(rr, req)

	// Should return 500 error
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for connection error, got %v", rr.Code)
	}

	// Should contain error message
	responseBody := rr.Body.String()
	if !strings.Contains(responseBody, "error") {
		t.Errorf("Expected error message in response, got %v", responseBody)
	}

	// Should be JSON error response
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Expected application/json Content-Type, got %v", rr.Header().Get("Content-Type"))
	}
}