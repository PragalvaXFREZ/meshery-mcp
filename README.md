# Meshery MCP Server

Thin MCP bridge (~650 LOC) for Meshery relationship workflows. Delegates to Meshery's REST API instead of reimplementing functionality locally.

## Tools

| Tool | Description |
|------|-------------|
| `get_schema` | Returns relationship schema version, valid kinds, required fields, and a minimal skeleton template |
| `get_model` | Fetches all component kinds registered under a Meshery model (e.g. `kubernetes`) |
| `manage_relationship` | Validates or creates a relationship. `action=validate` checks JSON against schema + component cross-reference. `action=create` generates a new relationship from the skeleton template |

## Prerequisites

A running Meshery server (default: `http://localhost:9081`).

## Build

```bash
go build -o bin/meshery-mcp ./cmd/meshery-mcp
```

## Run (stdio)

```bash
go run ./cmd/meshery-mcp --meshery-url http://localhost:9081
```

## Run (SSE)

```bash
go run ./cmd/meshery-mcp \
  --transport sse \
  --meshery-url http://localhost:9081 \
  --host 127.0.0.1 \
  --port 8080 \
  --endpoint /mcp
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--transport` | `stdio` | Transport mode: `stdio` or `sse` |
| `--meshery-url` | `http://localhost:9081` | Meshery server URL |
| `--host` | `127.0.0.1` | Host for SSE mode |
| `--port` | `8080` | Port for SSE mode |
| `--endpoint` | `/mcp` | HTTP endpoint path for SSE mode |
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--version` | | Print version and exit |

## Architecture

```
LLM Client
  │
  │ MCP Protocol (stdio or SSE)
  ▼
┌──────────────────────────┐
│   Meshery MCP Server     │
│                          │
│  3 tools, flat schemas   │
│  HTTP client ────────┐   │
└──────────────────────┤───┘
                       ▼
             ┌──────────────────┐
             │  Meshery Server  │
             │  (REST API)      │
             └──────────────────┘
```

The server is stateless. No local model files, no catalogs, no indexes. Component data and validation context come from Meshery's API.
