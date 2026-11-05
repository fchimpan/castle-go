package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// oapi-codegen resolves named components but cannot name these nested schema references.
var schemaReferences = map[string]string{
	"#/components/schemas/AggregationBase/properties/filters/allOf/0/oneOf/1":                      "AggregationLogicalFilter",
	"#/components/schemas/AggregationBase/properties/filters/allOf/0/oneOf/1/properties/$or/items": "AggregationFilterOperand",
	"#/components/schemas/BaseQuery/properties/filters":                                            "EventFilters",
	"#/components/schemas/BaseQuery/properties/filters/oneOf/1":                                    "EventLogicalFilter",
	"#/components/schemas/BaseQuery/properties/filters/oneOf/1/properties/$or/items":               "EventFilterOperand",
	"#/components/schemas/ListItemCount/properties/filters":                                        "ListItemFilters",
	"#/components/schemas/ListItemCount/properties/filters/oneOf/1":                                "ListItemLogicalFilter",
	"#/components/schemas/ListItemCount/properties/filters/oneOf/1/properties/$or/items":           "ListItemFilterOperand",
	"#/components/schemas/ListItemQueryWithSort/properties/filters/oneOf/0/items":                  "ListItemFilterCondition",
	"#/components/schemas/ListQuery/properties/filters/oneOf/1":                                    "ListLogicalFilter",
	"#/components/schemas/ListQuery/properties/filters/oneOf/1/properties/$or/items":               "ListFilterOperand",
	"#/components/schemas/LoginResponse/allOf/0":                                                   "AssessmentResponse",
	"#/paths/~1ips~1{value}/get/responses/200/content/application~1json/schema":                    "IPIntelligenceRecord",
}

func hoistSchemas(doc, schemas map[string]any) error {
	targets := make(map[string]map[string]any)
	for ref, name := range schemaReferences {
		if _, exists := schemas[name]; exists {
			return fmt.Errorf("hoisted schema name already exists: %s", name)
		}
		target, err := schemaAtPointer(doc, ref)
		if err != nil {
			return err
		}
		data, err := json.Marshal(target)
		if err != nil {
			return err
		}
		var copy map[string]any
		if err := json.Unmarshal(data, &copy); err != nil {
			return err
		}
		schemas[name] = copy
		targets[name] = target
	}
	for name, target := range targets {
		clear(target)
		target["$ref"] = "#/components/schemas/" + name
	}
	return rewriteReferences(doc)
}

func schemaAtPointer(doc map[string]any, ref string) (map[string]any, error) {
	var value any = doc
	for token := range strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch current := value.(type) {
		case map[string]any:
			value = current[token]
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(current) {
				return nil, fmt.Errorf("schema reference changed: %s", ref)
			}
			value = current[index]
		default:
			return nil, fmt.Errorf("schema reference changed: %s", ref)
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema reference is not an object: %s", ref)
	}
	return object, nil
}

func rewriteReferences(value any) error {
	switch value := value.(type) {
	case map[string]any:
		if ref, ok := value["$ref"].(string); ok {
			if name, exists := schemaReferences[ref]; exists {
				value["$ref"] = "#/components/schemas/" + name
			} else if len(strings.Split(ref, "/")) > 4 {
				return fmt.Errorf("unhandled nested schema reference: %s", ref)
			}
		}
		for _, child := range value {
			if err := rewriteReferences(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := rewriteReferences(child); err != nil {
				return err
			}
		}
	}
	return nil
}
