# Meshery MCP Server (Reference Implementation)

Minimal, extensible MCP server for Meshery relationship workflows:
- `create_relationship`
- `validate_relationship`
- `explain_relationship`

This implementation is schema/model-grounded and deterministic-first:
- Loads relationship schema from Meshery artifacts.
- Indexes model catalogs from local model paths.
- Validates relationships deterministically before final output.
- Uses optional integrations spreadsheet only as non-authoritative ranking hints.

## Architecture

- Knowledge Layer
  - Relationship schema loader (`v1alpha3`).
  - Model catalog indexer (`model.json`, `components/`, `relationships/`).
  - Optional integrations metadata loader (XLSX hints only).
- Deterministic Engine
  - Required/type/enum checks.
  - Selector structure checks.
  - Component reference resolution against indexed catalogs.
  - Structured error objects with stable codes:
    - `MissingField`
    - `InvalidEnum`
    - `InvalidType`
    - `ComponentNotFound`
    - `SelectorTooBroad`
    - `VersionMismatch`
    - `ParseError`
- LLM Orchestration Layer (reference heuristic implementation)
  - Intent-to-relationship proposal logic.
  - Propose -> validate -> repair loop.
- MCP Tools
  - `get_relationship_schema`
  - `index_model_catalog`
  - `list_components`
  - `validate_relationship`
  - `create_relationship`
  - `repair_relationship`
  - `explain_relationship`
  - `suggest_model`

## Build

```bash
go build ./cmd/meshery-mcp
```

## Run (stdio)

```bash
go run ./cmd/meshery-mcp \
  --transport stdio \
  --model-path ./testdata/sample-model
```

## Run (SSE)

```bash
go run ./cmd/meshery-mcp \
  --transport sse \
  --host 127.0.0.1 \
  --port 8080 \
  --endpoint /mcp \
  --model-path ./testdata/sample-model
```

## Key flags

- `--transport`: `stdio` or `sse`
- `--model-path`: default model catalog root
- `--schema-path`: relationship schema file path
- `--template-path`: relationship template path
- `--integrations-metadata-path`: optional spreadsheet path (hints only)

## Notes

- This is a coherent reference server, not a full product.
- For strict, current Meshery schema behavior, point `--schema-path` and `--template-path` to `meshery-schemas` artifacts.
- If model path is omitted during validation, schema checks run and component cross-reference is skipped with warnings.
