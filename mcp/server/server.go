package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "ghfs-mcp-server"
	ServerVersion = "0.1.0"
)

// NewServer creates a new MCP server with all GHFS tools registered.
func NewServer(ghfsURL string, debug bool) *mcp.Server {
	h := NewHandler(ghfsURL)

	s := mcp.NewServer(
		&mcp.Implementation{Name: ServerName, Version: ServerVersion},
		&mcp.ServerOptions{
			Instructions: fmt.Sprintf(
				"This MCP server provides tools to interact with a GHFS (Go HTTP File Server) instance at %s. "+
					"Available operations: list directories, read files, upload files/directories, create directories, delete files/directories, "+
					"generate archive download URLs.",
				ghfsURL,
			),
		},
	)

	if debug {
		s.AddReceivingMiddleware(debugMiddleware)
	}

	mcp.AddTool(s, ListTool, h.HandleList)
	mcp.AddTool(s, ReadTool, h.HandleRead)
	mcp.AddTool(s, UploadTool, h.HandleUpload)
	mcp.AddTool(s, MkdirTool, h.HandleMkdir)
	mcp.AddTool(s, DeleteTool, h.HandleDelete)
	mcp.AddTool(s, ArchiveTool, h.HandleArchive)

	return s
}

// debugMiddleware logs every incoming MCP request and its result.
func debugMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		params, _ := json.MarshalIndent(req.GetParams(), "", "  ")
		log.Printf("[DEBUG] --> %s\n%s", method, params)

		res, err := next(ctx, method, req)
		if err != nil {
			log.Printf("[DEBUG] <-- %s ERROR: %v", method, err)
			return res, err
		}

		result, _ := json.MarshalIndent(res, "", "  ")
		log.Printf("[DEBUG] <-- %s\n%s", method, result)
		return res, nil
	}
}
