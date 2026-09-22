package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func (s *Server) runGatewaySystemOneDecodeNormalizeHooks(ctx context.Context, call CallContext, headers http.Header, req *SystemOneRequest) error {
	return s.runGatewayDecodeNormalizeHooks(ctx, call, headers, *req, func(data json.RawMessage) error {
		return applySystemOneRequestPatch(req, data)
	})
}

func (s *Server) runGatewaySystemOnePrivacyPreHooks(ctx context.Context, call CallContext, headers http.Header, req *SystemOneRequest) error {
	return s.runGatewayPrivacyPreHooks(ctx, call, headers, *req, func(data json.RawMessage) error {
		return applySystemOneRequestPatch(req, data)
	})
}

func (s *Server) runGatewaySystemOneGuardrailPreHooks(ctx context.Context, call CallContext, req *SystemOneRequest) error {
	return s.runGatewayGuardrailPreHooks(ctx, call, *req, systemOneGuardrailTargets(req), func(data json.RawMessage) error {
		return applySystemOneRequestPatch(req, data)
	})
}

func (s *Server) runGatewaySystemOneContextOptimizeHooks(ctx context.Context, call CallContext, req *SystemOneRequest) error {
	return s.runGatewayContextOptimizeHooks(ctx, call, *req, func(data json.RawMessage) error {
		return applySystemOneRequestPatch(req, data)
	})
}

func systemOneGuardrailTargets(req *SystemOneRequest) []guardrailTextTarget {
	targets := []guardrailTextTarget{}
	appendSystemOneJSONTargets(&targets, req.State, "state", func(next json.RawMessage) { req.State = next }, &req.unsafeRedaction)
	ids := make([]string, 0, len(req.Questions))
	for id := range req.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for index, id := range ids {
		question := req.Questions[id]
		prefix := fmt.Sprintf("questions.%d", index)
		// IDs and criteria keys are part of the response contract. Inspect them,
		// but fail closed if a policy asks to redact rather than silently rename.
		appendGuardrailStringTarget(&targets, id, prefix+".id", func(any) { req.unsafeRedaction = true })
		appendSystemOneJSONTargets(&targets, question.Instructions, prefix+".instructions", func(next json.RawMessage) {
			q := req.Questions[id]
			q.Instructions = next
			req.Questions[id] = q
		}, &req.unsafeRedaction)
		appendSystemOneJSONTargets(&targets, question.Criteria, prefix+".criteria", func(next json.RawMessage) {
			q := req.Questions[id]
			q.Criteria = next
			req.Questions[id] = q
		}, &req.unsafeRedaction)
	}
	return targets
}

func appendSystemOneJSONTargets(targets *[]guardrailTextTarget, raw json.RawMessage, id string, set func(json.RawMessage), unsafeRedaction *bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil && string(raw) != "null" {
		appendGuardrailStringTarget(targets, text, id, func(next any) {
			encoded, _ := json.Marshal(next)
			set(encoded)
		})
		return
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil && number != "" {
		// Inspect without converting to float64 or changing the original JSON.
		// A text mask cannot retain the numeric type, so reject required masking.
		blockMask := func(any) { *unsafeRedaction = true }
		appendGuardrailStringTarget(targets, number.String(), id, blockMask)
		if expanded := systemOneGuardrailIntegerText(number); expanded != number.String() {
			appendGuardrailStringTarget(targets, expanded, id+".integer", blockMask)
		}
		return
	}
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) == nil && array != nil {
		for index, child := range array {
			appendSystemOneJSONTargets(targets, child, fmt.Sprintf("%s.%d", id, index), func(next json.RawMessage) {
				array[index] = next
				encoded, _ := json.Marshal(array)
				set(encoded)
			}, unsafeRedaction)
		}
		return
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for index, key := range keys {
		path := fmt.Sprintf("%s.%d", id, index)
		appendGuardrailStringTarget(targets, key, path+".key", func(any) { *unsafeRedaction = true })
		appendSystemOneJSONTargets(targets, object[key], path+".value", func(next json.RawMessage) {
			object[key] = next
			encoded, _ := json.Marshal(object)
			set(encoded)
		}, unsafeRedaction)
	}
}

func systemOneGuardrailIntegerText(number json.Number) string {
	// Also inspect integer-valued scientific notation, which can encode phone
	// numbers and numeric IDs. Bound exponent parsing before any expansion.
	original := number.String()
	coefficient, power := original, 0
	if index := strings.IndexAny(coefficient, "eE"); index >= 0 {
		exponent := coefficient[index+1:]
		coefficient = coefficient[:index]
		negative := strings.HasPrefix(exponent, "-")
		exponent = strings.TrimLeft(strings.TrimLeft(exponent, "+-"), "0")
		if len(exponent) > 18 {
			return original
		}
		if exponent != "" {
			var err error
			power, err = strconv.Atoi(exponent)
			if err != nil {
				return original
			}
		}
		if negative {
			power = -power
		}
		// Decimal places and trailing zeros cannot offset a larger exponent.
		// Check before shifting to keep arithmetic within the input's bounds.
		if power > len(coefficient)+64 || power < -len(coefficient) {
			return original
		}
	}
	negative := strings.HasPrefix(coefficient, "-")
	coefficient = strings.TrimPrefix(coefficient, "-")
	if index := strings.IndexByte(coefficient, '.'); index >= 0 {
		power -= len(coefficient) - index - 1
		coefficient = coefficient[:index] + coefficient[index+1:]
	}
	coefficient = strings.TrimLeft(coefficient, "0")
	if coefficient == "" {
		return "0"
	}
	trimmed := strings.TrimRight(coefficient, "0")
	power += len(coefficient) - len(trimmed)
	coefficient = trimmed
	if negative {
		coefficient = "-" + coefficient
	}
	if power < 0 || power > 64 || len(coefficient) > 64-power {
		return original
	}
	return coefficient + strings.Repeat("0", power)
}
