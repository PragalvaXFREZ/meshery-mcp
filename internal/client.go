package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// MesheryClient wraps HTTP calls to Meshery server API.
type MesheryClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

type componentsResponse struct {
	TotalCount int `json:"total_count"`
	Components []struct {
		Component struct {
			Kind string `json:"kind"`
		} `json:"component"`
	} `json:"components"`
}

// GetComponentKinds fetches all component kinds for the given model from Meshery.
// Returns deduplicated, sorted kind names and the total count.
func (c *MesheryClient) GetComponentKinds(modelName string) ([]string, int, error) {
	url := fmt.Sprintf("%s/api/meshmodels/models/%s/components?trim=true&pagesize=all", c.BaseURL, modelName)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot reach Meshery at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("Meshery returned status %d: %s", resp.StatusCode, string(body))
	}

	var result componentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	seen := make(map[string]bool)
	for _, comp := range result.Components {
		if comp.Component.Kind != "" {
			seen[comp.Component.Kind] = true
		}
	}

	kinds := make([]string, 0, len(seen))
	for k := range seen {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	return kinds, result.TotalCount, nil
}
