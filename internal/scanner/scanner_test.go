package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSingleFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	root, err := Scan(ScanOptions{Root: tmpFile})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if root.IsDir {
		t.Error("expected file, got directory")
	}
	if root.Size != 5 {
		t.Errorf("expected size 5, got %d", root.Size)
	}
	if root.Name != "test.txt" {
		t.Errorf("expected name test.txt, got %s", root.Name)
	}
}

func TestScanEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	root, err := Scan(ScanOptions{Root: dir})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if !root.IsDir {
		t.Error("expected directory")
	}
	if len(root.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(root.Children))
	}
}

func TestScanDirectoryWithFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bb"), 0644)

	root, err := Scan(ScanOptions{Root: dir})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if !root.IsDir {
		t.Error("expected directory")
	}
	if len(root.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(root.Children))
	}
	if root.Size != 5 { // 3 + 2
		t.Errorf("expected total size 5, got %d", root.Size)
	}
}

func TestScanNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)

	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)

	root, err := Scan(ScanOptions{Root: dir})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(root.Children) != 2 { // sub dir + root.txt
		t.Errorf("expected 2 children, got %d", len(root.Children))
	}

	// Find the subdirectory
	var sub *FileInfo
	for _, c := range root.Children {
		if c.Name == "sub" {
			sub = c
			break
		}
	}
	if sub == nil {
		t.Fatal("sub directory not found")
	}
	if len(sub.Children) != 1 {
		t.Errorf("expected 1 child in sub, got %d", len(sub.Children))
	}
	if sub.Size != 6 { // "nested" = 6 bytes
		t.Errorf("expected sub size 6, got %d", sub.Size)
	}
}

func TestScanWithExcludes(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0644)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(dir, "node_modules", "pkg.js"), []byte("package"), 0644)

	// Without excludes
	root, err := Scan(ScanOptions{Root: dir, UseExcludes: false})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(root.Children) != 2 {
		t.Errorf("expected 2 children without excludes, got %d", len(root.Children))
	}

	// With excludes
	root, err = Scan(ScanOptions{Root: dir, UseExcludes: true, Exclude: DefaultExcludes})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(root.Children) != 1 {
		t.Errorf("expected 1 child with excludes, got %d", len(root.Children))
	}
	if root.Children[0].Name != "keep.txt" {
		t.Errorf("expected keep.txt, got %s", root.Children[0].Name)
	}
}

func TestScanCustomExcludes(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0644)
	os.MkdirAll(filepath.Join(dir, ".cache"), 0755)
	os.WriteFile(filepath.Join(dir, ".cache", "data.bin"), []byte("data"), 0644)

	root, err := Scan(ScanOptions{
		Root:        dir,
		UseExcludes: true,
		Exclude:     []string{".cache"},
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(root.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(root.Children))
	}
	if root.Children[0].Name != "keep.txt" {
		t.Errorf("expected keep.txt, got %s", root.Children[0].Name)
	}
}

func TestScanMaxDepth(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0755)
	os.WriteFile(filepath.Join(dir, "a", "b", "c", "deep.txt"), []byte("deep"), 0644)

	// No limit
	root, err := Scan(ScanOptions{Root: dir})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// With MaxDepth=1 (root + 1 level)
	root, err = Scan(ScanOptions{Root: dir, MaxDepth: 1})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(root.Children) != 1 {
		t.Errorf("expected 1 child at depth 1, got %d", len(root.Children))
	}
	// The child "a" should have no children because we stopped at depth 1
	if len(root.Children[0].Children) != 0 {
		t.Errorf("expected 0 grandchildren at depth 1, got %d", len(root.Children[0].Children))
	}
}

func TestScanNonExistent(t *testing.T) {
	_, err := Scan(ScanOptions{Root: "/nonexistent/path/that/does/not/exist"})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestFlatten(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0644)

	root, err := Scan(ScanOptions{Root: dir})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	flat := Flatten(root)
	// root + sub dir + a.txt + b.txt = 4
	if len(flat) != 4 {
		t.Errorf("expected 4 flattened items, got %d", len(flat))
	}
}

func TestScanSorting(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "large.txt"), make([]byte, 100), 0644)
	os.MkdirAll(filepath.Join(dir, "zdir"), 0755)
	os.WriteFile(filepath.Join(dir, "zdir", "file.txt"), []byte("z"), 0644)

	root, err := Scan(ScanOptions{Root: dir})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Directories should come first
	if !root.Children[0].IsDir {
		t.Error("expected first child to be a directory")
	}
	// Files should be sorted by size descending
	files := []*FileInfo{}
	for _, c := range root.Children {
		if !c.IsDir {
			files = append(files, c)
		}
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Size < files[1].Size {
		t.Error("expected files sorted by size descending")
	}
}
