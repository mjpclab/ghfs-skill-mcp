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

// urlFor returns the URL of path on the server. Escaping keeps characters such
// as "?" and "#" in file names from being read as a query or fragment.
func (c *ghfsClient) urlFor(path string) string {
	return c.baseURL + (&url.URL{Path: path}).EscapedPath()
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
	u := c.urlFor(normalizePath(path))
	if sort != "" {
		u += "?sort=" + url.QueryEscape(sort)
	}

	var out ListOutput
	if err := c.getJSON(u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// fileContent is a file read from GHFS. data is nil when the file is over the size limit.
type fileContent struct {
	url      string
	mimeType string
	size     int64
	data     []byte
}

// read downloads the file at path, refusing directories. A file larger than
// maxSize is not downloaded; only its metadata is returned.
func (c *ghfsClient) read(path string, maxSize int64) (*fileContent, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("%s is a directory", path)
	}
	u := c.urlFor(path)

	// With Accept: application/json GHFS returns the entry's metadata instead of its content.
	var meta struct {
		Item ListItem `json:"item"`
	}
	if err := c.getJSON(u, &meta); err != nil {
		return nil, err
	}
	if meta.Item.IsDir {
		return nil, fmt.Errorf("%s is a directory", path)
	}

	// The content request also supplies the Content-Type, so it is sent even
	// for an oversized file, whose body is then left unread.
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	f := &fileContent{url: u, mimeType: resp.Header.Get("Content-Type"), size: meta.Item.Size}
	if f.size > maxSize {
		return f, nil
	}

	// The limit guards against the file growing between the two requests.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) <= maxSize {
		f.size, f.data = int64(len(data)), data
	}
	return f, nil
}

// maxJSONSize caps a JSON response; even very large directory listings fit well within it.
const maxJSONSize = 32 << 20

// getJSON sends a GET with Accept: application/json and decodes the response into v.
func (c *ghfsClient) getJSON(u string, v any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONSize+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxJSONSize {
		return fmt.Errorf("response exceeds %d bytes", maxJSONSize)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// upload uploads one or more files to the given path on GHFS.
// Files with "/" in their filepath are uploaded as "dirfile" (auto-creating subdirectories).
// Files without "/" in their filepath are uploaded as "file" (flat).
func (c *ghfsClient) upload(path string, files []uploadFile) error {
	path = normalizePath(path)
	u := c.urlFor(path) + "?upload"

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
	u := c.urlFor(path) + "?mkdir"

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
	u := c.urlFor(path) + "?delete"

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
	u := c.urlFor(path) + "?" + url.QueryEscape(format)
	if filename != "" {
		u += "=" + url.QueryEscape(filename)
	}
	for _, name := range names {
		u += "&name=" + url.QueryEscape(name)
	}
	return u
}
