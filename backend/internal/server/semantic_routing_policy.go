package server

import (
	"encoding/json"
	"math"
	"net/http"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const semanticRoutingMetadataKey = "tokenhub_semantic_routing"

// SemanticRoutingPolicy is an optional overlay on the model's base algorithm.
// Enabling it explicitly permits semantic selection within this public model.
type SemanticRoutingPolicy struct {
	Mode          string  `json:"mode"`
	MinConfidence float64 `json:"min_confidence"`
}

func (p *SemanticRoutingPolicy) UnmarshalJSON(data []byte) error {
	var raw struct {
		Mode          string   `json:"mode"`
		MinConfidence *float64 `json:"min_confidence"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.MinConfidence == nil {
		return NewHTTPError(http.StatusBadRequest, "invalid_semantic_routing_policy", "Semantic routing requires an explicit confidence threshold")
	}
	*p = SemanticRoutingPolicy{Mode: raw.Mode, MinConfidence: *raw.MinConfidence}
	return validateSemanticRoutingPolicy(p)
}

func validateSemanticRoutingPolicy(policy *SemanticRoutingPolicy) error {
	if policy == nil {
		return nil
	}
	if (policy.Mode != "off" && policy.Mode != "shadow" && policy.Mode != "enforce") ||
		math.IsNaN(policy.MinConfidence) || math.IsInf(policy.MinConfidence, 0) || policy.MinConfidence < 0 || policy.MinConfidence > 1 {
		return NewHTTPError(http.StatusBadRequest, "invalid_semantic_routing_policy", "Semantic routing requires mode off, shadow, or enforce and a confidence threshold between 0 and 1")
	}
	return nil
}

func modelSemanticRoutingPolicy(model Model) SemanticRoutingPolicy {
	policy := SemanticRoutingPolicy{Mode: "off", MinConfidence: 0.65}
	if raw := model.Metadata[semanticRoutingMetadataKey]; raw != "" {
		if json.Unmarshal([]byte(raw), &policy) != nil || validateSemanticRoutingPolicy(&policy) != nil {
			return SemanticRoutingPolicy{Mode: "off", MinConfidence: 0.65}
		}
	}
	return policy
}

func saveSemanticRoutingPolicy(tx *gorm.DB, modelName string, policy *SemanticRoutingPolicy) error {
	if policy == nil {
		return nil
	}
	var model Model
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&model, "name = ?", modelName).Error; err != nil {
		return notFound(err, "model_not_found", "Model not found")
	}
	if model.Metadata == nil {
		model.Metadata = map[string]string{}
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	model.Metadata[semanticRoutingMetadataKey] = string(encoded)
	return tx.Model(&model).Select("Metadata").Updates(&model).Error
}

// Only the routing policy endpoint may replace this managed field on an existing
// model. Ordinary edits and catalog imports can carry stale or absent metadata.
func preserveSemanticRoutingMetadata(current, incoming map[string]string) map[string]string {
	if incoming == nil && current[semanticRoutingMetadataKey] == "" {
		return nil
	}
	metadata := make(map[string]string, len(incoming)+1)
	for key, value := range incoming {
		if key != semanticRoutingMetadataKey {
			metadata[key] = value
		}
	}
	if value, ok := current[semanticRoutingMetadataKey]; ok {
		metadata[semanticRoutingMetadataKey] = value
	}
	return metadata
}
