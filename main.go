package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"github.com/google/uuid"
)

const RESULTS_DIR = "hurl_results"

// Middleware to check API token
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiToken := os.Getenv("API_TOKEN")
		if apiToken == "" {
			log.Fatal("API_TOKEN environment variable not set")
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != apiToken {
			http.Error(w, "Invalid API token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func runHurl(content string, resultDir string) error {
	// Write content to hurl file
	hurlFile := filepath.Join(resultDir, "test.hurl")
	if err := os.WriteFile(hurlFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write hurl file: %v", err)
	}

	// Prepare command with --test flag
	cmd := exec.Command("hurl", "--test", hurlFile)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	// Combine stdout and stderr
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Write output to result file
	outputFile := filepath.Join(resultDir, "result.txt")
	if writeErr := os.WriteFile(outputFile, []byte(output), 0644); writeErr != nil {
		return fmt.Errorf("failed to write result file: %v", writeErr)
	}

	return err
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if len(content) == 0 {
		http.Error(w, "Request body cannot be empty", http.StatusBadRequest)
		return
	}

	// Generate UUID and create directory
	id := uuid.New().String()
	resultDir := filepath.Join(RESULTS_DIR, id)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		http.Error(w, "Failed to create result directory", http.StatusInternalServerError)
		return
	}

	// Run hurl asynchronously
	go func() {
		if err := runHurl(string(content), resultDir); err != nil {
			log.Printf("Error running hurl for %s: %v", id, err)
		}
	}()

	// Return UUID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func handleResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract UUID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	id := parts[2]

	// Validate UUID format
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "Invalid UUID format", http.StatusBadRequest)
		return
	}

	// Check if result file exists
	resultFile := filepath.Join(RESULTS_DIR, id, "result.txt")
	if _, err := os.Stat(resultFile); os.IsNotExist(err) {
		// Check if directory exists to determine if job is pending or not found
		if _, err := os.Stat(filepath.Join(RESULTS_DIR, id)); os.IsNotExist(err) {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
		return
	}

	// Read and return result file
	content, err := os.ReadFile(resultFile)
	if err != nil {
		http.Error(w, "Failed to read result file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if hurl is available
	if err := exec.Command("hurl", "--version").Run(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("hurl not available"))
		return
	}

	w.Write([]byte("ok"))
}

func main() {
	// Ensure results directory exists
	if err := os.MkdirAll(RESULTS_DIR, 0755); err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Apply auth middleware to all endpoints except health
	http.HandleFunc("/run", authMiddleware(handleRun))
	http.HandleFunc("/results/", authMiddleware(handleResults))
	http.HandleFunc("/health", handleHealth)

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}