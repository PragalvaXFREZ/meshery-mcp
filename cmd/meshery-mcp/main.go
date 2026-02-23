package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/meshery/meshery-mcp/internal"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var (
		transport  = flag.String("transport", "stdio", "transport mode: stdio or sse")
		mesheryURL = flag.String("meshery-url", "http://localhost:9081", "Meshery server URL")
		host       = flag.String("host", "127.0.0.1", "host for SSE mode")
		port       = flag.Int("port", 8080, "port for SSE mode")
		endpoint   = flag.String("endpoint", "/mcp", "HTTP endpoint path for SSE mode")
		logLevel   = flag.String("log-level", "info", "log level: debug, info, warn, error")
		version    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *version {
		fmt.Println("meshery-mcp 0.2.0")
		return
	}

	logger := buildLogger(*logLevel)
	server := internal.BuildServer(*mesheryURL, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch strings.ToLower(strings.TrimSpace(*transport)) {
	case "stdio":
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && err != context.Canceled {
			log.Fatalf("stdio server failed: %v", err)
		}
	case "sse":
		h := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server { return server }, nil)
		mux := http.NewServeMux()
		mux.Handle(*endpoint, h)

		addr := fmt.Sprintf("%s:%d", *host, *port)
		httpServer := &http.Server{Addr: addr, Handler: mux}

		go func() {
			<-ctx.Done()
			_ = httpServer.Shutdown(context.Background())
		}()

		log.Printf("Meshery MCP SSE server listening on http://%s%s", addr, *endpoint)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("sse server failed: %v", err)
		}
	default:
		log.Fatalf("unsupported transport %q, expected stdio or sse", *transport)
	}
}

func buildLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}
