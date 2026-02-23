package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const relationshipSkeleton = `{
  "schemaVersion": "relationships.meshery.io/v1alpha3",
  "version": "v1.0.0",
  "kind": "",
  "type": "",
  "subType": "",
  "status": "enabled",
  "model": {"name": ""},
  "selectors": [{"allow": {"from": [{"kind": "", "model": {"name": ""}}], "to": [{"kind": "", "model": {"name": ""}}]}}],
  "metadata": {}
}`

// --- get_schema ---

type GetSchemaInput struct{}

type GetSchemaOutput struct {
	SchemaVersion  string   `json:"schema_version"`
	ValidKinds     []string `json:"valid_kinds"`
	RequiredFields []string `json:"required_fields"`
	Skeleton       string   `json:"skeleton"`
}

func handleGetSchema(_ context.Context, _ *mcp.CallToolRequest, _ GetSchemaInput) (*mcp.CallToolResult, GetSchemaOutput, error) {
	out := GetSchemaOutput{
		SchemaVersion:  v1alpha3.RelationshipSchemaVersion,
		ValidKinds:     []string{"edge", "hierarchical", "sibling"},
		RequiredFields: []string{"schemaVersion", "version", "kind", "type", "subType", "model", "selectors"},
		Skeleton:       relationshipSkeleton,
	}
	return nil, out, nil
}

// --- get_model ---

type GetModelInput struct {
	ModelName  string `json:"model_name"`
	MesheryURL string `json:"meshery_url,omitempty"`
}

type GetModelOutput struct {
	ModelName      string   `json:"model_name"`
	ComponentKinds []string `json:"component_kinds"`
	Total          int      `json:"total"`
}

func makeGetModelHandler(defaultClient *MesheryClient) mcp.ToolHandlerFor[GetModelInput, GetModelOutput] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in GetModelInput) (*mcp.CallToolResult, GetModelOutput, error) {
		client := defaultClient
		if in.MesheryURL != "" {
			client = &MesheryClient{
				BaseURL:    in.MesheryURL,
				HTTPClient: &http.Client{Timeout: 30 * time.Second},
			}
		}

		kinds, total, err := client.GetComponentKinds(in.ModelName)
		if err != nil {
			return nil, GetModelOutput{}, err
		}

		return nil, GetModelOutput{
			ModelName:      in.ModelName,
			ComponentKinds: kinds,
			Total:          total,
		}, nil
	}
}

// --- manage_relationship ---

type ManageRelationshipInput struct {
	Action           string `json:"action"`
	RelationshipJSON string `json:"relationship_json,omitempty"`
	ModelName        string `json:"model_name,omitempty"`
	Kind             string `json:"kind,omitempty"`
	SubType          string `json:"sub_type,omitempty"`
	Type             string `json:"type,omitempty"`
	FromKind         string `json:"from_kind,omitempty"`
	ToKind           string `json:"to_kind,omitempty"`
	MesheryURL       string `json:"meshery_url,omitempty"`
}

type ManageRelationshipOutput struct {
	Relationship string            `json:"relationship,omitempty"`
	Valid        bool              `json:"valid"`
	Errors       []ValidationError `json:"errors,omitempty"`
	Warnings     []ValidationError `json:"warnings,omitempty"`
}

func makeManageRelationshipHandler(defaultClient *MesheryClient) mcp.ToolHandlerFor[ManageRelationshipInput, ManageRelationshipOutput] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in ManageRelationshipInput) (*mcp.CallToolResult, ManageRelationshipOutput, error) {
		client := defaultClient
		if in.MesheryURL != "" {
			client = &MesheryClient{
				BaseURL:    in.MesheryURL,
				HTTPClient: &http.Client{Timeout: 30 * time.Second},
			}
		}

		switch in.Action {
		case "validate":
			return handleValidate(in, client)
		case "create":
			return handleCreate(in, client)
		default:
			return nil, ManageRelationshipOutput{}, fmt.Errorf("unknown action %q: must be \"validate\" or \"create\"", in.Action)
		}
	}
}

func handleValidate(in ManageRelationshipInput, client *MesheryClient) (*mcp.CallToolResult, ManageRelationshipOutput, error) {
	if in.RelationshipJSON == "" {
		return nil, ManageRelationshipOutput{}, fmt.Errorf("relationship_json is required for validate action")
	}

	vr := ValidateRelationship(in.RelationshipJSON, client, in.ModelName)
	return nil, ManageRelationshipOutput{
		Valid:    vr.Valid,
		Errors:   vr.Errors,
		Warnings: vr.Warnings,
	}, nil
}

func handleCreate(in ManageRelationshipInput, client *MesheryClient) (*mcp.CallToolResult, ManageRelationshipOutput, error) {
	var relDef relationship.RelationshipDefinition
	if err := json.Unmarshal([]byte(relationshipSkeleton), &relDef); err != nil {
		return nil, ManageRelationshipOutput{}, fmt.Errorf("internal error: %w", err)
	}

	if in.Kind != "" {
		relDef.Kind = relationship.RelationshipDefinitionKind(in.Kind)
	}
	if in.Type != "" {
		relDef.RelationshipType = in.Type
	}
	if in.SubType != "" {
		relDef.SubType = in.SubType
	}
	if in.ModelName != "" {
		relDef.Model.Name = in.ModelName
	}

	id, err := relDef.GenerateID()
	if err != nil {
		return nil, ManageRelationshipOutput{}, fmt.Errorf("failed to generate ID: %w", err)
	}
	relDef.Id = &id

	// Marshal to JSON
	relJSON, err := json.Marshal(&relDef)
	if err != nil {
		return nil, ManageRelationshipOutput{}, fmt.Errorf("failed to marshal relationship: %w", err)
	}

	// If from/to kinds were provided, rebuild selectors via map
	if in.FromKind != "" || in.ToKind != "" {
		var m map[string]interface{}
		_ = json.Unmarshal(relJSON, &m)

		fromEntry := map[string]interface{}{"kind": in.FromKind}
		if in.ModelName != "" {
			fromEntry["model"] = map[string]interface{}{"name": in.ModelName}
		}
		toEntry := map[string]interface{}{"kind": in.ToKind}
		if in.ModelName != "" {
			toEntry["model"] = map[string]interface{}{"name": in.ModelName}
		}

		selector := map[string]interface{}{
			"allow": map[string]interface{}{
				"from": []interface{}{fromEntry},
				"to":   []interface{}{toEntry},
			},
		}
		m["selectors"] = []interface{}{selector}

		relJSON, err = json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, ManageRelationshipOutput{}, fmt.Errorf("failed to marshal relationship: %w", err)
		}
	}

	relStr := string(relJSON)

	// Validate the created relationship
	vr := ValidateRelationship(relStr, client, in.ModelName)

	return nil, ManageRelationshipOutput{
		Relationship: relStr,
		Valid:        vr.Valid,
		Errors:       vr.Errors,
		Warnings:     vr.Warnings,
	}, nil
}

// RegisterTools registers all 3 tools on the MCP server.
func RegisterTools(server *mcp.Server, client *MesheryClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_schema",
		Description: "Returns the Meshery relationship schema version, valid kinds, required fields, and a minimal skeleton template. Use this first to understand the relationship format.",
	}, handleGetSchema)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_model",
		Description: "Fetches all component kinds registered under a Meshery model (e.g. 'kubernetes'). Use this to discover valid component kinds before creating or validating relationships.",
	}, makeGetModelHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "manage_relationship",
		Description: "Validates or creates a Meshery relationship. Use action='validate' with relationship_json to check an existing relationship. Use action='create' with kind, type, sub_type, and optionally from_kind/to_kind to generate a new relationship from the skeleton template.",
	}, makeManageRelationshipHandler(client))
}
