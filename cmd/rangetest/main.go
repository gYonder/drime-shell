package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	baseURL  = "https://app.drime.cloud/api/v1"
	fileHash = "NDg2NDY1MzMwfA"
)

type Config struct {
	Token string `yaml:"token"`
}

func loadToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(home, ".drime-shell", "config.yaml"))
	if err != nil {
		return "", err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", err
	}

	return cfg.Token, nil
}

func main() {
	token, err := loadToken()
	if err != nil {
		fmt.Printf("Failed to load token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Token loaded: %s...%s\n\n", token[:8], token[len(token)-4:])

	url := fmt.Sprintf("%s/file-entries/download/%s", baseURL, fileHash)

	// Test 1: HEAD request to check Accept-Ranges header
	fmt.Println("=== Test 1: HEAD request ===")
	testHead(url, token)

	// Test 2: GET request with Range header (first 100 bytes)
	fmt.Println("\n=== Test 2: Range request (bytes=0-99) ===")
	testRange(url, token, "bytes=0-99")

	// Test 3: GET request with Range header (skip first 100 bytes)
	fmt.Println("\n=== Test 3: Range request (bytes=100-199) ===")
	testRange(url, token, "bytes=100-199")

	// Test 4: GET request with open-ended Range (resume from byte 1000)
	fmt.Println("\n=== Test 4: Range request (bytes=1000-) - resume style ===")
	testRange(url, token, "bytes=1000-")
}

func testHead(url, token string) {
	req, _ := http.NewRequest("HEAD", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Content-Length: %d\n", resp.ContentLength)
	fmt.Printf("Accept-Ranges: %q\n", resp.Header.Get("Accept-Ranges"))
	fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))

	if resp.Header.Get("Accept-Ranges") == "bytes" {
		fmt.Println("✅ Server advertises Range support!")
	} else if resp.Header.Get("Accept-Ranges") == "" {
		fmt.Println("⚠️  No Accept-Ranges header (might still support it)")
	} else {
		fmt.Println("❌ Server does not support Range requests")
	}
}

func testRange(url, token, rangeHeader string) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Range", rangeHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Content-Length: %d\n", resp.ContentLength)
	fmt.Printf("Content-Range: %q\n", resp.Header.Get("Content-Range"))
	fmt.Printf("Accept-Ranges: %q\n", resp.Header.Get("Accept-Ranges"))

	// Read a small sample of the body
	sample := make([]byte, 50)
	n, _ := io.ReadAtLeast(resp.Body, sample, 50)
	if n > 0 {
		fmt.Printf("Body sample (first %d bytes): %q\n", n, sample[:n])
	}

	switch resp.StatusCode {
	case 206:
		fmt.Println("✅ 206 Partial Content - Range requests ARE supported!")
	case 200:
		fmt.Println("⚠️  200 OK - Server ignored Range header (not supported or full file returned)")
	default:
		fmt.Printf("❓ Unexpected status: %d\n", resp.StatusCode)
	}
}
