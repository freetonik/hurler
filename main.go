package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func runHurl(content string) (string, error) {
	tempDir, err := os.MkdirTemp("", "hurl-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write content to temporary file
	hurlFile := filepath.Join(tempDir, "test.hurl")
	if err := os.WriteFile(hurlFile, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write hurl file: %v", err)
	}

	// Prepare command with --test flag
	cmd := exec.Command("hurl", "--test", hurlFile)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err = cmd.Run()

	// Always return stdout+stderr regardless of error
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	return output, err
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body as plain text
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if len(content) == 0 {
		http.Error(w, "Request body cannot be empty", http.StatusBadRequest)
		return
	}

	// Run hurl
	output, err := runHurl(string(content))

	// Set plain text response
	w.Header().Set("Content-Type", "text/plain")

	if err != nil {
		// Return error status code but still include the output
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Write([]byte(output))
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/run", handleRun)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}