package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler holds the GHFS client and provides MCP tool handler functions.
//
// Each handler receives its input already unmarshaled and validated against the
// tool's input schema, so it only checks constraints the schema cannot express.
// A returned error becomes a tool error result rather than a protocol error.
type Handler struct {
	client *ghfsClient
}

// NewHandler creates a new Handler with the given GHFS base URL.
func NewHandler(ghfsURL string) *Handler {
	return &Handler{client: newGHFSClient(ghfsURL)}
}

// textResult wraps a plain text message as a tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// HandleList handles the ghfs_list tool call.
// The SDK puts the output in StructuredContent and its JSON encoding in Content.
func (h *Handler) HandleList(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, ListOutput, error) {
	out, err := h.client.list(in.Path, in.Sort)
	if err != nil {
		return nil, ListOutput{}, fmt.Errorf("failed to list directory: %w", err)
	}
	return nil, *out, nil
}

// maxReadSize caps how much of a file ghfs_read returns, to protect the model's context.
const maxReadSize = 1 << 20

// imageTypes are the image formats ghfs_read hands to the model as images.
var imageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// HandleRead handles the ghfs_read tool call. Text is returned in the output,
// images as image content, and other binary files only as a link, whatever their size.
func (h *Handler) HandleRead(ctx context.Context, req *mcp.CallToolRequest, in ReadInput) (*mcp.CallToolResult, ReadOutput, error) {
	f, err := h.client.read(in.Path, maxReadSize)
	if err != nil {
		return nil, ReadOutput{}, fmt.Errorf("failed to read file: %w", err)
	}

	mediaType, params, _ := mime.ParseMediaType(f.mimeType)
	isImage := slices.Contains(imageTypes, mediaType)
	out := ReadOutput{Path: in.Path, Size: f.size, MIMEType: f.mimeType, URL: f.url}

	if f.data == nil {
		// Text and images are worth reading but too big; anything else just gets a link.
		if isImage || strings.HasPrefix(mediaType, "text/") || params["charset"] != "" {
			return nil, ReadOutput{}, fmt.Errorf("%s is %d bytes, over the %d byte limit; download it from %s", in.Path, f.size, maxReadSize, f.url)
		}
		return binaryResult(f, mediaType, out)
	}

	switch {
	case isImage:
		out.Kind = "image"
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("%s: %s image, %d bytes", in.Path, mediaType, f.size)},
			&mcp.ImageContent{Data: f.data, MIMEType: mediaType},
		}}, out, nil

	case utf8.Valid(f.data):
		out.Kind, out.Content = "text", string(f.data)
		return nil, out, nil

	default:
		return binaryResult(f, mediaType, out)
	}
}

// binaryResult describes a binary file by its metadata and a link, without its content.
func binaryResult(f *fileContent, mediaType string, out ReadOutput) (*mcp.CallToolResult, ReadOutput, error) {
	out.Kind = "binary"
	return &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: fmt.Sprintf("%s: binary file (%s, %d bytes), content not returned. Download it from %s", out.Path, f.mimeType, f.size, f.url)},
		&mcp.ResourceLink{URI: f.url, Name: path.Base(out.Path), MIMEType: mediaType, Size: &f.size},
	}}, out, nil
}

// HandleUpload handles the ghfs_upload tool call.
func (h *Handler) HandleUpload(ctx context.Context, req *mcp.CallToolRequest, in UploadInput) (*mcp.CallToolResult, any, error) {
	if len(in.Files) == 0 {
		return nil, nil, fmt.Errorf("parameter 'files' must not be empty")
	}

	uploadFiles := make([]uploadFile, 0, len(in.Files))
	for i, f := range in.Files {
		if f.Filepath == "" {
			return nil, nil, fmt.Errorf("files[%d]: 'filepath' must not be empty", i)
		}

		content, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("files[%d]: invalid base64 content: %w", i, err)
		}

		uploadFiles = append(uploadFiles, uploadFile{
			filepath: f.Filepath,
			content:  content,
		})
	}

	if err := h.client.upload(in.Path, uploadFiles); err != nil {
		return nil, nil, fmt.Errorf("upload failed: %w", err)
	}

	return textResult(fmt.Sprintf("Successfully uploaded %d file(s) to %s", len(uploadFiles), in.Path)), nil, nil
}

// HandleMkdir handles the ghfs_mkdir tool call.
func (h *Handler) HandleMkdir(ctx context.Context, req *mcp.CallToolRequest, in MkdirInput) (*mcp.CallToolResult, any, error) {
	if len(in.Names) == 0 {
		return nil, nil, fmt.Errorf("parameter 'names' must not be empty")
	}

	if err := h.client.mkdir(in.Path, in.Names); err != nil {
		return nil, nil, fmt.Errorf("mkdir failed: %w", err)
	}

	return textResult(fmt.Sprintf("Successfully created %d directory(ies) under %s: %v", len(in.Names), in.Path, in.Names)), nil, nil
}

// HandleDelete handles the ghfs_delete tool call.
func (h *Handler) HandleDelete(ctx context.Context, req *mcp.CallToolRequest, in DeleteInput) (*mcp.CallToolResult, any, error) {
	if len(in.Names) == 0 {
		return nil, nil, fmt.Errorf("parameter 'names' must not be empty")
	}

	if err := h.client.delete(in.Path, in.Names); err != nil {
		return nil, nil, fmt.Errorf("delete failed: %w", err)
	}

	return textResult(fmt.Sprintf("Successfully deleted %d item(s) from %s: %v", len(in.Names), in.Path, in.Names)), nil, nil
}

// HandleArchive handles the ghfs_archive tool call.
func (h *Handler) HandleArchive(ctx context.Context, req *mcp.CallToolRequest, in ArchiveInput) (*mcp.CallToolResult, any, error) {
	if !slices.Contains(archiveFormats, any(in.Format)) {
		return nil, nil, fmt.Errorf("invalid format %q: must be \"tar\", \"tgz\", or \"zip\"", in.Format)
	}

	downloadURL := h.client.archiveURL(in.Path, in.Format, in.Names, in.Filename)

	result := fmt.Sprintf("Archive download URL:\n%s\n\nFormat: %s", downloadURL, in.Format)
	if len(in.Names) > 0 {
		result += fmt.Sprintf("\nItems: %v", in.Names)
	} else {
		result += fmt.Sprintf("\nScope: entire directory %s", in.Path)
	}
	if in.Filename != "" {
		result += fmt.Sprintf("\nFilename: %s", in.Filename)
	}

	return textResult(result), nil, nil
}
