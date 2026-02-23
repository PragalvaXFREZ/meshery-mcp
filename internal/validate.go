package internal

import (
	"strings"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
)

// ValidationError describes a single validation problem.
type ValidationError struct {
	Field      string `json:"field"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ValidationResult holds the outcome of relationship validation.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// ValidateRelationship performs 2-phase validation on a relationship JSON string.
// Phase 1: schema checks (always runs).
// Phase 2: component cross-reference (runs only if client and modelName are provided).
func ValidateRelationship(jsonStr string, client *MesheryClient, modelName string) ValidationResult {
	result := ValidationResult{Valid: true}

	var relDef relationship.RelationshipDefinition
	if err := encoding.Unmarshal([]byte(jsonStr), &relDef); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "root",
			Message: "failed to parse relationship JSON: " + err.Error(),
		})
		return result
	}

	// Phase 1: Schema checks
	if relDef.SchemaVersion != v1alpha3.RelationshipSchemaVersion {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "schemaVersion",
			Message:    "invalid schemaVersion: " + relDef.SchemaVersion,
			Suggestion: v1alpha3.RelationshipSchemaVersion,
		})
	}

	switch relDef.Kind {
	case relationship.Edge, relationship.Hierarchical, relationship.Sibling:
		// valid
	default:
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:      "kind",
			Message:    "invalid kind: " + string(relDef.Kind),
			Suggestion: "must be one of: edge, hierarchical, sibling",
		})
	}

	if relDef.Model.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "model.name",
			Message: "model name is required",
		})
	}

	if relDef.Selectors == nil || len(*relDef.Selectors) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "selectors",
			Message: "at least one selector is required",
		})
	} else {
		for i, sel := range *relDef.Selectors {
			if len(sel.Allow.From) == 0 && len(sel.Allow.To) == 0 {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "selectors",
					Message: selectorMsg(i, "must have at least one 'from' or 'to' entry"),
				})
			}

			// Check patch consistency
			for _, from := range sel.Allow.From {
				if from.Patch != nil && (from.Patch.PatchStrategy == nil || *from.Patch.PatchStrategy == "") {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   "selectors",
						Message: selectorMsg(i, "from patch is set but patchStrategy is missing"),
					})
				}
			}
			for _, to := range sel.Allow.To {
				if to.Patch != nil && (to.Patch.PatchStrategy == nil || *to.Patch.PatchStrategy == "") {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   "selectors",
						Message: selectorMsg(i, "to patch is set but patchStrategy is missing"),
					})
				}
			}
		}
	}

	if relDef.SubType == "" {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "subType",
			Message: "subType is empty; consider setting it for better classification",
		})
	}

	// Phase 2: Component cross-reference
	if !result.Valid || client == nil || modelName == "" {
		return result
	}

	kinds, _, err := client.GetComponentKinds(modelName)
	if err != nil {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "components",
			Message: "could not fetch components for cross-reference: " + err.Error(),
		})
		return result
	}

	kindSet := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		kindSet[k] = true
	}

	if relDef.Selectors != nil {
		for _, sel := range *relDef.Selectors {
			for _, from := range sel.Allow.From {
				if from.Kind == nil || *from.Kind == "" || *from.Kind == "*" {
					continue
				}
				// Check model name matches for from side
				if from.Model != nil && from.Model.Name != "" && from.Model.Name != modelName {
					continue
				}
				if !kindSet[*from.Kind] {
					suggestion := findClosestKind(*from.Kind, kindSet)
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:      "selectors.allow.from.kind",
						Message:    "component kind not found in model: " + *from.Kind,
						Suggestion: suggestion,
					})
				}
			}
			for _, to := range sel.Allow.To {
				if to.Kind == nil || *to.Kind == "" || *to.Kind == "*" {
					continue
				}
				// Only error if to model is empty or matches modelName (allow cross-model)
				if to.Model != nil && to.Model.Name != "" && to.Model.Name != modelName {
					continue
				}
				if !kindSet[*to.Kind] {
					suggestion := findClosestKind(*to.Kind, kindSet)
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:      "selectors.allow.to.kind",
						Message:    "component kind not found in model: " + *to.Kind,
						Suggestion: suggestion,
					})
				}
			}
		}
	}

	return result
}

func selectorMsg(i int, msg string) string {
	return "selector[" + itoa(i) + "]: " + msg
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// findClosestKind finds the closest matching kind from the set using
// case-insensitive match, prefix match, then Levenshtein distance.
func findClosestKind(target string, kinds map[string]bool) string {
	lower := strings.ToLower(target)

	// Case-insensitive exact match
	for k := range kinds {
		if strings.ToLower(k) == lower {
			return k
		}
	}

	// Prefix match
	for k := range kinds {
		if strings.HasPrefix(strings.ToLower(k), lower) || strings.HasPrefix(lower, strings.ToLower(k)) {
			return k
		}
	}

	// Levenshtein distance
	best := ""
	bestDist := -1
	for k := range kinds {
		d := levenshtein(lower, strings.ToLower(k))
		if bestDist < 0 || d < bestDist {
			bestDist = d
			best = k
		}
	}
	if bestDist >= 0 && bestDist <= len(target)/2+1 {
		return best
	}
	return ""
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
