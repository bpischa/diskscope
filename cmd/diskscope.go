package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/bpischa/diskscope/internal/scanner"
)

type scanRequest struct {
	Path        string   `json:"path"`
	FullDisk    bool     `json:"fullDisk"`
	UseExcludes bool     `json:"useExcludes"`
	ExcludeList []string `json:"excludeList"`
}

type scanResponse struct {
	Root     *scanner.FileInfo `json:"root"`
	Duration string            `json:"duration"`
}

func main() {
	port := flag.String("port", "8765", "Port to listen on")
	flag.Parse()

	http.HandleFunc("/api/scan", handleScan)
	http.HandleFunc("/api/elevate", handleElevate)
	http.Handle("/", http.FileServer(http.Dir("web/static")))

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("DiskScope running at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		req.Path = home
	}

	// If full disk requested, trigger elevation
	if req.FullDisk && runtime.GOOS != "windows" {
		// Check if we're already root
		if os.Geteuid() != 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "elevation_required",
				"message": "Full disk scan requires elevated permissions. Click 'Elevate' to restart with sudo.",
			})
			return
		}
	}

	doScan(w, req.Path, req.UseExcludes, req.ExcludeList)
}

func doScan(w http.ResponseWriter, path string, useExcludes bool, excludeList []string) {
	excludes := scanner.DefaultExcludes
	if len(excludeList) > 0 {
		excludes = excludeList
	}

	opts := scanner.ScanOptions{
		Root:        path,
		Exclude:     excludes,
		UseExcludes: useExcludes,
	}

	root, err := scanner.Scan(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scanResponse{
		Root: root,
	})
}

func handleElevate(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS == "windows" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "On Windows, please right-click the app and select 'Run as administrator'",
		})
		return
	}

	// Re-exec self with sudo
	self, err := os.Executable()
	if err != nil {
		http.Error(w, "Cannot find executable: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("sudo", self)
	cmd.Env = append(os.Environ(),
		"DISKSCOPE_ELEVATED=1",
		"DISKSCOPE_PATH="+r.URL.Query().Get("path"),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to elevate: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "elevating",
		"message": "Restarting with elevated permissions...",
	})
}
