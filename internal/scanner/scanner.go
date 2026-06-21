package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultExcludes are common directories users typically want to skip.
var DefaultExcludes = []string{
	"node_modules",
	".git",
	"__pycache__",
	".venv",
	"venv",
	".tox",
	".next",
	".nuxt",
	"dist-newstyle",
	".stack-work",
	"target/debug",
	"target/release",
	".Trash",
}

// FileInfo represents a file or directory node in the tree.
type FileInfo struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Size     int64      `json:"size"`
	IsDir    bool       `json:"isDir"`
	Children []*FileInfo `json:"children,omitempty"`
}

// ScanOptions controls the scanning behavior.
type ScanOptions struct {
	Root      string
	Exclude   []string
	UseExcludes bool
	MaxDepth  int
}

// Scan walks the directory tree and returns the root FileInfo.
func Scan(opts ScanOptions) (*FileInfo, error) {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}

	root := &FileInfo{
		Name:  info.Name(),
		Path:  absRoot,
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		root.Size = info.Size()
		return root, nil
	}

	excludeSet := make(map[string]struct{}, len(opts.Exclude))
	for _, e := range opts.Exclude {
		excludeSet[strings.TrimSpace(e)] = struct{}{}
	}

	var totalSize atomic.Int64
	scanDir(root, absRoot, opts, excludeSet, 0, &totalSize)
	root.Size = totalSize.Load()

	return root, nil
}

func scanDir(node *FileInfo, path string, opts ScanOptions, excludeSet map[string]struct{}, depth int, totalSize *atomic.Int64) {
	if opts.MaxDepth > 0 && depth >= opts.MaxDepth {
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	type entryResult struct {
		info *FileInfo
		size int64
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make([]entryResult, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		if opts.UseExcludes {
			if _, excluded := excludeSet[name]; excluded {
				continue
			}
		}

		fullPath := filepath.Join(path, name)
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		child := &FileInfo{
			Name:  name,
			Path:  fullPath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var dirSize atomic.Int64
				scanDir(child, fullPath, opts, excludeSet, depth+1, &dirSize)
				child.Size = dirSize.Load()
				totalSize.Add(child.Size)
				mu.Lock()
				results = append(results, entryResult{child, dirSize.Load()})
				mu.Unlock()
			}()
		} else {
			sz := fi.Size()
			child.Size = sz
			totalSize.Add(sz)
			results = append(results, entryResult{child, 0})
		}
	}

	wg.Wait()

	// Sort: directories first, then by size descending
	sort.Slice(results, func(i, j int) bool {
		if results[i].info.IsDir != results[j].info.IsDir {
			return results[i].info.IsDir
		}
		return results[i].info.Size > results[j].info.Size
	})

	for _, r := range results {
		node.Children = append(node.Children, r.info)
	}
}

// Flatten converts the tree into a flat list suitable for D3 treemap.
func Flatten(root *FileInfo) []*FileInfo {
	var result []*FileInfo
	var walk func(n *FileInfo)
	walk = func(n *FileInfo) {
		result = append(result, n)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return result
}
