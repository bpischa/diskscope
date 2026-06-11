package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

	// If elevated scan was requested via restart, handle it
	if os.Getenv("DISKSCOPE_ELEVATED") == "1" {
		path := os.Getenv("DISKSCOPE_PATH")
		if path == "" {
			path = "/"
		}
		doScanAndServe(path, true)
		return
	}

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
		home, _ := os.UserHomeDir()
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

func doScanAndServe(path string, useExcludes bool) {
	excludes := scanner.DefaultExcludes
	opts := scanner.ScanOptions{
		Root:        path,
		Exclude:     excludes,
		UseExcludes: useExcludes,
	}
	root, err := scanner.Scan(opts)
	if err != nil {
		log.Fatal(err)
	}
	w := &nopResponseWriter{}
	json.NewEncoder(w).Encode(scanResponse{Root: root})
}

type nopResponseWriter struct{}

func (n *nopResponseWriter) Header() http.Header { return http.Header{} }
func (n *nopResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopResponseWriter) WriteHeader(code int) {}

func handleElevate(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS == "windows" {
		http.Error(w, "On Windows, please restart the app as Administrator", http.StatusBadRequest)
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

func getDisks() []string {
	var disks []string
	switch runtime.GOOS {
	case "darwin":
		entries, _ := os.ReadDir("/Volumes")
		for _, e := range entries {
			disks = append(disks, filepath.Join("/Volumes", e.Name()))
		}
		disks = append(disks, "/")
	case "linux":
		// Read /proc/mounts or /etc/mtab
		data, err := os.ReadFile("/proc/mounts")
		if err == nil {
			seen := map[string]bool{}
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					mount := parts[1]
					if strings.HasPrefix(mount, "/") && !seen[mount] {
						seen[mount] = true
						disks = append(disks, mount)
					}
				}
			}
		}
	case "windows":
		for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			path := string(drive) + ":\\"
			if _, err := os.Stat(path); err == nil {
				disks = append(disks, path)
			}
		}
	}
	return disks
}
