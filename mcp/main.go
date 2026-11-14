package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	mcpserver "ghfs-mcp-server/server"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ghfsURL := flag.String("ghfs-url", "http://localhost:8080", "GHFS server base URL")
	mode := flag.String("mode", "stdio", "Run mode: stdio or http")
	certFile := flag.String("cert", "", "TLS certificate file (enables HTTPS in http mode)")
	keyFile := flag.String("key", "", "TLS private key file (enables HTTPS in http mode)")
	addr := flag.String("addr", "", "HTTP listen address, only for http mode (default \":8080\", or \":8443\" with TLS)")
	debug := flag.Bool("debug", false, "Enable debug logging of MCP messages")
	flag.Parse()

	useTLS := *certFile != "" && *keyFile != ""

	if *addr == "" {
		if useTLS {
			*addr = ":8443"
		} else {
			*addr = ":8080"
		}
	}

	s := mcpserver.NewServer(*ghfsURL, *debug)

	switch *mode {
	case "stdio":
		log.Printf("Starting %s %s in STDIO mode (GHFS: %s)\n", mcpserver.ServerName, mcpserver.ServerVersion, *ghfsURL)
		if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("STDIO server error: %v", err)
		}

	case "http":
		// Stateless is required for protocol 2026-07-28; the tools never call back to the client.
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true})
		log.Printf("Starting %s %s in HTTP mode on %s (GHFS: %s)\n", mcpserver.ServerName, mcpserver.ServerVersion, *addr, *ghfsURL)

		var err error
		if useTLS {
			log.Println("Using HTTPS scheme")
			err = http.ListenAndServeTLS(*addr, *certFile, *keyFile, handler)
		} else {
			err = http.ListenAndServe(*addr, handler)
		}
		if err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (expected 'stdio' or 'http')\n", *mode)
		os.Exit(1)
	}
}
