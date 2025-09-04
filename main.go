package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type FlareSolverrRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
}

type FlareSolverrResponse struct {
	Solution struct {
		Response  string        `json:"response"`
		Status    int           `json:"status"`
		Cookies   []interface{} `json:"cookies"`
		UserAgent string        `json:"userAgent"`
	} `json:"solution"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ProxyHandler struct {
	flareSolverrURL string
	client          *http.Client
}

func NewProxyHandler() *ProxyHandler {
	flareSolverrURL := os.Getenv("FLARESOLVERR_URL")
	if flareSolverrURL == "" {
		flareSolverrURL = "http://flaresolverr:8191/v1"
	}

	return &ProxyHandler{
		flareSolverrURL: flareSolverrURL,
		client:          &http.Client{},
	}
}

type DirectHandler struct {
	flareSolverrURL string
	client          *http.Client
}

func NewDirectHandler() *DirectHandler {
	flareSolverrURL := os.Getenv("FLARESOLVERR_URL")
	if flareSolverrURL == "" {
		flareSolverrURL = "http://flaresolverr:8191/v1"
	}

	return &DirectHandler{
		flareSolverrURL: flareSolverrURL,
		client:          &http.Client{},
	}
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p.handleRequest(w, r)
	case http.MethodConnect:
		// CONNECT method is not supported as this is an HTTP-only proxy adapter
		// that uses FlareSolverr to bypass Cloudflare protection.
		// Clients should use HTTP URLs even for HTTPS sites.
		p.sendConnectError(w)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *ProxyHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	url := r.URL.String()
	// Convert HTTP to HTTPS for FlareSolverr
	url = strings.Replace(url, "http://", "https://", 1)

	requestData := FlareSolverrRequest{
		Cmd:        "request.get",
		URL:        url,
		MaxTimeout: 60000,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		p.sendError(w, fmt.Sprintf("Failed to marshal request: %v", err))
		return
	}

	req, err := http.NewRequest("POST", p.flareSolverrURL, bytes.NewBuffer(jsonData))
	if err != nil {
		p.sendError(w, fmt.Sprintf("Failed to create request: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		p.sendError(w, fmt.Sprintf("Failed to connect to FlareSolverr: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.sendError(w, fmt.Sprintf("Failed to read response: %v", err))
		return
	}

	var flareResponse FlareSolverrResponse
	if err := json.Unmarshal(body, &flareResponse); err != nil {
		p.sendError(w, fmt.Sprintf("Failed to parse response: %v", err))
		return
	}

	if flareResponse.Status != "ok" {
		p.sendError(w, fmt.Sprintf("FlareSolverr error: %s", flareResponse.Message))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(flareResponse.Solution.Response))
}

func (p *ProxyHandler) sendError(w http.ResponseWriter, message string) {
	log.Printf("Error: %s", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	errorResponse := map[string]string{"error": message}
	json.NewEncoder(w).Encode(errorResponse)
}

func (p *ProxyHandler) sendConnectError(w http.ResponseWriter) {
	message := "CONNECT method is not supported. This is an HTTP-only proxy adapter for FlareSolverr. " +
		"Please use HTTP URLs (e.g., http://example.com) even for HTTPS sites. " +
		"The proxy will automatically handle HTTPS conversion when communicating with FlareSolverr."

	log.Printf("CONNECT rejected: %s", message)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(message))
}

func (d *DirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse the URL from the path
	// Format: /domain.com/path/to/resource
	path := r.URL.Path
	if path == "/" || path == "" {
		http.Error(w, "Invalid URL format. Use: http://localhost:PORT/domain.com/path", http.StatusBadRequest)
		return
	}

	// Remove leading slash
	if path[0] == '/' {
		path = path[1:]
	}

	// Find the first slash to separate domain from path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		http.Error(w, "Invalid URL format. Use: http://localhost:PORT/domain.com/path", http.StatusBadRequest)
		return
	}

	domain := parts[0]
	remainingPath := ""
	if len(parts) > 1 {
		remainingPath = "/" + parts[1]
	}

	// Add query parameters if present
	if r.URL.RawQuery != "" {
		remainingPath += "?" + r.URL.RawQuery
	}

	// Construct the target URL (try HTTPS first)
	targetURL := "https://" + domain + remainingPath

	// Determine the FlareSolverr command based on HTTP method
	var cmd string
	switch r.Method {
	case http.MethodGet:
		cmd = "request.get"
	case http.MethodPost:
		cmd = "request.post"
	default:
		// For other methods, default to request.get
		// FlareSolverr may not support all methods
		cmd = "request.get"
		log.Printf("Warning: HTTP method %s may not be fully supported by FlareSolverr, using request.get", r.Method)
	}

	// Forward the request through FlareSolverr
	d.forwardToFlareSolverr(w, targetURL, cmd)
}

func (d *DirectHandler) forwardToFlareSolverr(w http.ResponseWriter, targetURL string, cmd string) {
	requestData := FlareSolverrRequest{
		Cmd:        cmd,
		URL:        targetURL,
		MaxTimeout: 60000,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		d.sendError(w, fmt.Sprintf("Failed to marshal request: %v", err))
		return
	}

	req, err := http.NewRequest("POST", d.flareSolverrURL, bytes.NewBuffer(jsonData))
	if err != nil {
		d.sendError(w, fmt.Sprintf("Failed to create request: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		d.sendError(w, fmt.Sprintf("Failed to connect to FlareSolverr: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.sendError(w, fmt.Sprintf("Failed to read response: %v", err))
		return
	}

	var flareResponse FlareSolverrResponse
	if err := json.Unmarshal(body, &flareResponse); err != nil {
		d.sendError(w, fmt.Sprintf("Failed to parse response: %v", err))
		return
	}

	if flareResponse.Status != "ok" {
		// If HTTPS fails, try HTTP as fallback
		if strings.HasPrefix(targetURL, "https://") {
			httpURL := strings.Replace(targetURL, "https://", "http://", 1)
			log.Printf("HTTPS failed, trying HTTP fallback for: %s", httpURL)
			d.forwardToFlareSolverr(w, httpURL, cmd)
			return
		}
		d.sendError(w, fmt.Sprintf("FlareSolverr error: %s", flareResponse.Message))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(flareResponse.Solution.Response))
}

func (d *DirectHandler) sendError(w http.ResponseWriter, message string) {
	log.Printf("Error: %s", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	errorResponse := map[string]string{"error": message}
	json.NewEncoder(w).Encode(errorResponse)
}

func main() {
	// Get FlareSolverr URL for logging
	flareSolverrURL := os.Getenv("FLARESOLVERR_URL")
	if flareSolverrURL == "" {
		flareSolverrURL = "http://flaresolverr:8191/v1"
	}
	log.Printf("FlareSolverr URL: %s", flareSolverrURL)

	// Start direct routing server (primary service)
	directHandler := NewDirectHandler()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	directServer := &http.Server{
		Addr:    ":" + port,
		Handler: directHandler,
	}

	log.Printf("FlareProxy adapter (direct mode) running on port %s", port)
	log.Printf("Direct mode usage: http://localhost:%s/domain.com/path", port)

	// Start proxy server if PROXY_PORT is configured
	proxyPort := os.Getenv("PROXY_PORT")
	if proxyPort != "" {
		proxyHandler := NewProxyHandler()
		proxyServer := &http.Server{
			Addr:    ":" + proxyPort,
			Handler: proxyHandler,
		}

		log.Printf("FlareProxy adapter (proxy mode) running on port %s", proxyPort)
		log.Printf("Proxy mode usage: Set http://localhost:%s as HTTP proxy", proxyPort)

		// Run proxy server in a goroutine
		go func() {
			if err := proxyServer.ListenAndServe(); err != nil {
				log.Fatalf("Proxy server error: %v", err)
			}
		}()
	}

	// Run direct server (blocks)
	if err := directServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
