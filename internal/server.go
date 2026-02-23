package internal

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// BuildServer creates a configured MCP server with all tools registered.
func BuildServer(mesheryURL string, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "meshery-mcp",
		Version: "0.2.0",
	}, &mcp.ServerOptions{
		Logger:       logger,
		Instructions: "Meshery relationship assistant. Use get_schema for reference, get_model for components, manage_relationship to validate or create.",
	})

	client := &MesheryClient{
		BaseURL:    mesheryURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	RegisterTools(server, client)
	return server
}
