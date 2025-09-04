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

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodConnect:
		p.handleRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *ProxyHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	url := r.URL.String()
	if r.Method == http.MethodConnect {
		url = "https://" + r.Host + r.URL.Path
	} else {
		url = strings.Replace(url, "http://", "https://", 1)
	}

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

func main() {
	handler := NewProxyHandler()
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	log.Printf("FlareProxy adapter running on port %s", port)
	log.Printf("FlareSolverr URL: %s", handler.flareSolverrURL)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
