package server

import (
	"fmt"
	"net/http"
	"testing"
)

const testBaseURL = "http://localhost:8080"
const testBasePath = "/ttt/"

func TestList(t *testing.T) {
	c := newGHFSClient(testBaseURL)
	out, err := c.list(testBasePath, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if out.Path != testBasePath {
		t.Errorf("path = %q, want %q", out.Path, testBasePath)
	}
	fmt.Printf("List %s: %+v\n", testBasePath, *out)
}

func TestMkdir(t *testing.T) {
	c := newGHFSClient(testBaseURL)
	err := c.mkdir(testBasePath, []string{"_test_mcp_dir"})
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	fmt.Printf("Mkdir %s_test_mcp_dir/ OK\n", testBasePath)
}

func TestUpload(t *testing.T) {
	c := newGHFSClient(testBaseURL)
	files := []uploadFile{
		{filepath: "hello.txt", content: []byte("Hello from MCP server test!")},
		{filepath: "subdir/nested.txt", content: []byte("Nested file content")},
	}
	err := c.upload(testBasePath+"_test_mcp_dir/", files)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	fmt.Printf("Upload to %s_test_mcp_dir/ OK\n", testBasePath)
}

func TestListAfterUpload(t *testing.T) {
	c := newGHFSClient(testBaseURL)
	out, err := c.list(testBasePath+"_test_mcp_dir/", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	got := map[string]bool{}
	for _, item := range out.SubItems {
		got[item.Name] = item.IsDir
	}
	if isDir, ok := got["hello.txt"]; !ok || isDir {
		t.Errorf("want file hello.txt in %+v", out.SubItems)
	}
	if isDir, ok := got["subdir"]; !ok || !isDir {
		t.Errorf("want directory subdir in %+v", out.SubItems)
	}
	fmt.Printf("List %s_test_mcp_dir/: %+v\n", testBasePath, *out)
}

func TestDelete(t *testing.T) {
	c := newGHFSClient(testBaseURL)
	err := c.delete(testBasePath, []string{"_test_mcp_dir"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	fmt.Printf("Delete %s_test_mcp_dir/ OK\n", testBasePath)
}

func TestArchiveURL(t *testing.T) {
	c := newGHFSClient(testBaseURL)

	// Test basic archive URL generation
	u := c.archiveURL(testBasePath, "zip", nil, "")
	expected := testBaseURL + testBasePath + "?zip"
	if u != expected {
		t.Fatalf("archiveURL mismatch:\n  got:  %s\n  want: %s", u, expected)
	}
	fmt.Printf("ArchiveURL (zip, no options): %s\n", u)

	// Test with custom filename
	u = c.archiveURL(testBasePath, "tgz", nil, "backup.tar.gz")
	expected = testBaseURL + testBasePath + "?tgz=backup.tar.gz"
	if u != expected {
		t.Fatalf("archiveURL mismatch:\n  got:  %s\n  want: %s", u, expected)
	}
	fmt.Printf("ArchiveURL (tgz, filename): %s\n", u)

	// Test with names
	u = c.archiveURL(testBasePath, "tar", []string{"a.txt", "b.txt"}, "")
	expected = testBaseURL + testBasePath + "?tar&name=a.txt&name=b.txt"
	if u != expected {
		t.Fatalf("archiveURL mismatch:\n  got:  %s\n  want: %s", u, expected)
	}
	fmt.Printf("ArchiveURL (tar, names): %s\n", u)

	// Test with names + filename
	u = c.archiveURL(testBasePath, "zip", []string{"dir1"}, "archive.zip")
	expected = testBaseURL + testBasePath + "?zip=archive.zip&name=dir1"
	if u != expected {
		t.Fatalf("archiveURL mismatch:\n  got:  %s\n  want: %s", u, expected)
	}
	fmt.Printf("ArchiveURL (zip, names+filename): %s\n", u)
}

func TestArchiveDownload(t *testing.T) {
	c := newGHFSClient(testBaseURL)

	// Verify the archive URL is actually downloadable
	u := c.archiveURL(testBasePath, "zip", nil, "")
	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("Archive download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Archive download returned HTTP %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	fmt.Printf("Archive download OK: status=%d, content-type=%s\n", resp.StatusCode, ct)
}
