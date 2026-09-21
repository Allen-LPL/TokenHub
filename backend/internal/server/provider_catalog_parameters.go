package server

import (
	"slices"
	"strings"
)

// Capability-only catalogs omit routine token budgets. Expand only an advertised
// endpoint's baseline wire field; explicit budget declarations remain authoritative.
func catalogBudgetParameters(parameters []string, endpoints string) []string {
	for _, budget := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
		if slices.Contains(parameters, budget) {
			return parameters
		}
	}
	for _, endpoint := range strings.Split(endpoints, ",") {
		switch strings.TrimPrefix(strings.TrimSpace(endpoint), "/v1/") {
		case "chat/completions":
			parameters = append(parameters, "max_tokens")
		case "responses":
			parameters = append(parameters, "max_output_tokens")
		}
	}
	return parameters
}
