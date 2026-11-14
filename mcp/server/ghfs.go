package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ghfsClient is an HTTP client for interacting with a GHFS server.
type ghfsClient struct {
	baseURL    string
	httpClient *http.Client
}

// uploadFile represents a file to be uploaded.
type uploadFile struct {
	filepath string // relative path, e.g. "file.txt" or "subdir/file.txt"
	content  []byte // file content
}

// newGHFSClient creates a new GHFS client with the given base URL.
func newGHFSClient(baseURL string) *ghfsClient {
	return &ghfsClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// normalizePath ensures the path starts with "/" and ends with "/".
func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	return path
}

// list retrieves the directory listing at the given path.
func (c *ghfsClient) list(path string, sort string) (*ListOutput, error) {
	path = normalizePath(path)

	u := c.baseURL + path
	if sort != "" {
		u += "?sort=" + url.QueryEscape(sort)
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out ListOutput
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode listing: %w", err)
	}
	return &out, nil
}

// upload uploads one or more files to the given path on GHFS.
// Files with "/" in their filepath are uploaded as "dirfile" (auto-creating subdirectories).
// Files without "/" in their filepath are uploaded as "file" (flat).
func (c *ghfsClient) upload(path string, files []uploadFile) error {
	path = normalizePath(path)
	u := c.baseURL + path + "?upload"

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for _, f := range files {
		fieldName := "file"
		if strings.Contains(f.filepath, "/") {
			fieldName = "dirfile"
		}

		part, err := writer.CreateFormFile(fieldName, f.filepath)
		if err != nil {
			return fmt.Errorf("create form file %q: %w", f.filepath, err)
		}
		if _, err := part.Write(f.content); err != nil {
			return fmt.Errorf("write content for %q: %w", f.filepath, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", u, &buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// mkdir creates one or more directories under the given path.
func (c *ghfsClient) mkdir(path string, names []string) error {
	path = normalizePath(path)
	u := c.baseURL + path + "?mkdir"

	form := url.Values{}
	for _, name := range names {
		form.Add("name", name)
	}

	req, err := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// delete deletes one or more files or directories under the given path.
func (c *ghfsClient) delete(path string, names []string) error {
	path = normalizePath(path)
	u := c.baseURL + path + "?delete"

	form := url.Values{}
	for _, name := range names {
		form.Add("name", name)
	}

	req, err := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// archiveURL constructs a download URL for archiving files at the given path.
func (c *ghfsClient) archiveURL(path string, format string, names []string, filename string) string {
	path = normalizePath(path)
	u := c.baseURL + path + "?" + url.QueryEscape(format)
	if filename != "" {
		u += "=" + url.QueryEscape(filename)
	}
	for _, name := range names {
		u += "&name=" + url.QueryEscape(name)
	}
	return u
}
