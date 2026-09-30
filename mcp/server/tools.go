package server

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The tool input schemas below are inferred from these structs: a field without
// "omitempty" becomes a required property, and the "jsonschema" tag supplies the
// property description.

// ListInput is the input for the ghfs_list tool.
type ListInput struct {
	Path string `json:"path" jsonschema:"Directory path to list, e.g. \"/\" or \"/docs/\""`
	Sort string `json:"sort,omitempty" jsonschema:"Sort order for directory listing. Key: \"n\" name, \"e\" extension/type, \"s\" size, \"t\" time, \"_\" no sort. Uppercase reverses order (e.g. \"N\" name desc). Prefix \"/\" puts dirs first (e.g. \"/n\"), suffix \"/\" puts dirs last (e.g. \"n/\"). Examples: \"/T\" (dirs first, time desc), \"n\" (name asc), \"S\" (size desc)."`
}

// UploadFileInput describes a single file within an upload request.
type UploadFileInput struct {
	Filepath string `json:"filepath" jsonschema:"Relative file path including filename. Use \"/\" for directory structure, e.g. \"subdir/file.txt\""`
	Content  string `json:"content" jsonschema:"Base64-encoded file content"`
}

// UploadInput is the input for the ghfs_upload tool.
type UploadInput struct {
	Path  string            `json:"path" jsonschema:"Target directory path on GHFS, e.g. \"/ttt/\""`
	Files []UploadFileInput `json:"files" jsonschema:"Files to upload. Each entry has \"filepath\" (relative path, e.g. \"file.txt\" or \"subdir/file.txt\") and \"content\" (base64-encoded file content)."`
}

// MkdirInput is the input for the ghfs_mkdir tool.
type MkdirInput struct {
	Path  string   `json:"path" jsonschema:"Parent directory path, e.g. \"/ttt/\""`
	Names []string `json:"names" jsonschema:"Directory names to create. Supports nested paths like \"foo/bar\"."`
}

// DeleteInput is the input for the ghfs_delete tool.
type DeleteInput struct {
	Path  string   `json:"path" jsonschema:"Parent directory path containing the items to delete, e.g. \"/ttt/\""`
	Names []string `json:"names" jsonschema:"Names of files or directories to delete."`
}

// ArchiveInput is the input for the ghfs_archive tool.
type ArchiveInput struct {
	Path     string   `json:"path" jsonschema:"Directory path to archive, e.g. \"/ttt/mydir/\""`
	Format   string   `json:"format" jsonschema:"Archive format: \"tar\", \"tgz\", or \"zip\""`
	Names    []string `json:"names,omitempty" jsonschema:"Optional: specific sub-item names to include in the archive. If omitted, the entire directory is archived."`
	Filename string   `json:"filename,omitempty" jsonschema:"Optional: custom filename for the downloaded archive"`
}

// archiveFormats lists the archive formats GHFS supports.
var archiveFormats = []any{"tar", "tgz", "zip"}

// schemaFor infers the JSON schema of T and applies tweaks that "jsonschema"
// struct tags cannot express, such as enum constraints. It panics on failure,
// since the schema is derived from a compile-time type.
func schemaFor[T any](tweak func(properties map[string]*jsonschema.Schema)) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("infer JSON schema for %T: %v", *new(T), err))
	}
	if tweak != nil {
		tweak(s.Properties)
	}
	return s
}

// ListTool defines the ghfs_list tool for listing directory contents.
var ListTool = &mcp.Tool{
	Name:        "ghfs_list",
	Description: "List directory contents on the GHFS server. Returns JSON with file/directory names, sizes, modification times, and types.",
}

// UploadTool defines the ghfs_upload tool for uploading files/directories.
var UploadTool = &mcp.Tool{
	Name:        "ghfs_upload",
	Description: "Upload one or more files to the GHFS server. Supports uploading directory structures by using relative paths with \"/\" in filepath. Files with \"/\" in filepath use \"dirfile\" mode, which creates missing subdirectories; that needs mkdir permission on the target path as well as upload, otherwise GHFS returns HTTP 500 and writes nothing.",
}

// MkdirTool defines the ghfs_mkdir tool for creating directories.
var MkdirTool = &mcp.Tool{
	Name:        "ghfs_mkdir",
	Description: "Create one or more directories on the GHFS server. Supports creating nested directories like \"foo/bar/baz\".",
}

// DeleteTool defines the ghfs_delete tool for deleting files/directories.
var DeleteTool = &mcp.Tool{
	Name:        "ghfs_delete",
	Description: "Delete one or more files or directories on the GHFS server. Directories are deleted recursively.",
}

// ArchiveTool defines the ghfs_archive tool for archiving files/directories.
var ArchiveTool = &mcp.Tool{
	Name:        "ghfs_archive",
	Description: "Archive (download as package) files or directories on the GHFS server. Returns a download URL for the archive. Supports tar, tgz and zip formats.",
	InputSchema: schemaFor[ArchiveInput](func(properties map[string]*jsonschema.Schema) {
		properties["format"].Enum = archiveFormats
	}),
}
