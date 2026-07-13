// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

// Command specdowngrade rewrites an OpenAPI 3.1 document into the 3.0 subset
// that oapi-codegen (kin-openapi) understands.
//
// The AgentOps control-plane spec is emitted by FastAPI as OpenAPI 3.1.0, which
// oapi-codegen v2 does not yet fully support (https://github.com/oapi-codegen/oapi-codegen/issues/373).
// The 3.1-only constructs that actually appear in our spec are:
//
//   - nullable via "anyOf"/"oneOf" containing a {"type":"null"} member
//     (Pydantic v2's representation of Optional[...]) -> 3.0 "nullable": true
//   - "type" as an array including "null"                -> single type + "nullable": true
//   - numeric "exclusiveMinimum"/"exclusiveMaximum"       -> 3.0 boolean form + minimum/maximum
//   - "const": x                                          -> "enum": [x]
//
// This is a codegen-time transform only; the vendored api/openapi.json stays a
// faithful 3.1 mirror of the monorepo spec so that `make sync-spec` diffs stay
// readable against upstream. See GNUmakefile's `generate` target.
//
// Usage: specdowngrade <input.json> <output.json>
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: specdowngrade <input.json> <output.json>")
		os.Exit(2)
	}
	in, out := os.Args[1], os.Args[2]

	raw, err := os.ReadFile(in)
	if err != nil {
		fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		fatal(fmt.Errorf("parse %s: %w", in, err))
	}

	// FastAPI stamps 3.1.0; the transformed document is a 3.0 subset.
	if v, ok := doc["openapi"].(string); ok && len(v) > 0 && v[0] == '3' {
		doc["openapi"] = "3.0.3"
	}

	downgrade(doc)

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fatal(err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(out, encoded, 0o644); err != nil {
		fatal(err)
	}
}

// downgrade walks the entire JSON tree and rewrites 3.1-only schema constructs
// in place. It is intentionally generic: any object may be a schema, so every
// object is normalized.
func downgrade(node any) {
	switch v := node.(type) {
	case map[string]any:
		normalizeSchemaObject(v)
		for _, child := range v {
			downgrade(child)
		}
	case []any:
		for _, child := range v {
			downgrade(child)
		}
	}
}

func normalizeSchemaObject(obj map[string]any) {
	collapseNullableCombinator(obj, "anyOf")
	collapseNullableCombinator(obj, "oneOf")
	normalizeTypeArray(obj)
	normalizeExclusiveBound(obj, "exclusiveMinimum", "minimum")
	normalizeExclusiveBound(obj, "exclusiveMaximum", "maximum")
	constToEnum(obj)
}

// collapseNullableCombinator turns `{"anyOf":[X, {"type":"null"}]}` into the 3.0
// nullable form. A {"type":"null"} member becomes `"nullable": true`; if a single
// non-null member remains it is inlined (a bare $ref is wrapped in allOf so the
// sibling nullable is preserved).
func collapseNullableCombinator(obj map[string]any, key string) {
	list, ok := obj[key].([]any)
	if !ok {
		return
	}

	kept := make([]any, 0, len(list))
	sawNull := false
	for _, item := range list {
		if m, ok := item.(map[string]any); ok && isNullOnly(m) {
			sawNull = true
			continue
		}
		kept = append(kept, item)
	}
	if !sawNull {
		return
	}

	obj["nullable"] = true

	switch len(kept) {
	case 0:
		delete(obj, key)
	case 1:
		delete(obj, key)
		only, ok := kept[0].(map[string]any)
		if !ok {
			obj[key] = kept
			return
		}
		if _, isRef := only["$ref"]; isRef {
			// A $ref's siblings are ignored in 3.0; wrap so nullable sticks.
			obj["allOf"] = []any{only}
			return
		}
		for k, val := range only {
			if _, exists := obj[k]; !exists {
				obj[k] = val
			}
		}
	default:
		obj[key] = kept
	}
}

// isNullOnly reports whether a schema is exactly {"type":"null"} (the Pydantic
// Optional sentinel), ignoring cosmetic-only keys.
func isNullOnly(m map[string]any) bool {
	t, ok := m["type"].(string)
	if !ok || t != "null" {
		return false
	}
	for k := range m {
		if k != "type" {
			return false
		}
	}
	return true
}

// normalizeTypeArray converts 3.1 `"type": ["string","null"]` into a single type
// plus `"nullable": true`.
func normalizeTypeArray(obj map[string]any) {
	arr, ok := obj["type"].([]any)
	if !ok {
		return
	}
	var nonNull []any
	for _, t := range arr {
		if s, ok := t.(string); ok && s == "null" {
			obj["nullable"] = true
			continue
		}
		nonNull = append(nonNull, t)
	}
	switch len(nonNull) {
	case 1:
		obj["type"] = nonNull[0]
	case 0:
		delete(obj, "type")
	default:
		obj["type"] = nonNull[0] // 3.0 allows only a single type
	}
}

// normalizeExclusiveBound converts 3.1's numeric exclusiveMinimum/Maximum into
// the 3.0 boolean form (minimum/maximum + exclusive*: true).
func normalizeExclusiveBound(obj map[string]any, excl, incl string) {
	switch obj[excl].(type) {
	case float64, json.Number:
		obj[incl] = obj[excl]
		obj[excl] = true
	}
}

// constToEnum rewrites 3.1 `"const": x` as `"enum": [x]`.
func constToEnum(obj map[string]any) {
	c, ok := obj["const"]
	if !ok {
		return
	}
	delete(obj, "const")
	if _, exists := obj["enum"]; !exists {
		obj["enum"] = []any{c}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "specdowngrade:", err)
	os.Exit(1)
}
