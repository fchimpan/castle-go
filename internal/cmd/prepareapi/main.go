// Command prepareapi applies documented generation-only adaptations to the snapshot.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && os.Args[1] == "-clean" {
		return os.Remove("api/prepared.tmp.json")
	}
	b, err := os.ReadFile("api/openapi.json")
	if err != nil {
		return err
	}
	metadata, err := os.ReadFile("api/source.json")
	if err != nil {
		return err
	}
	if err := verifySnapshot(b, metadata); err != nil {
		return err
	}
	out, err := prepare(b)
	if err != nil {
		return err
	}
	return os.WriteFile("api/prepared.tmp.json", out, 0600)
}

func prepare(b []byte) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	schemas, err := objectAt(doc, "components", "schemas")
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"RiskRequest", "FilterRequest"} {
		if err := allowSkippedValidation(schemas, name); err != nil {
			return nil, err
		}
	}
	properties, err := objectAt(schemas, "Changeset", "properties", "authentication_method.type", "oneOf", 1, "properties")
	if err != nil {
		return nil, err
	}
	enum, err := objectAt(schemas, "AuthenticationMethodType")
	if err != nil {
		return nil, err
	}
	values, ok := enum["enum"].([]any)
	if !ok || enum["type"] != "string" || len(values) == 0 {
		return nil, fmt.Errorf("authentication method enum changed")
	}
	for _, name := range []string{"from", "to"} {
		old, err := objectAt(properties, name)
		if err != nil {
			return nil, err
		}
		if old["$ref"] != "#/components/schemas/AuthenticationMethodType" {
			return nil, fmt.Errorf("authentication method %s changed", name)
		}
		properties[name] = map[string]any{"type": "string", "enum": enum["enum"], "nullable": true}
	}
	// The device allOf branch conflicts with its referenced schema's metadata.
	branch, err := objectAt(schemas, "LoginResponse", "allOf", 0, "properties", "device", "allOf", 1)
	if err != nil {
		return nil, err
	}
	if branch["readOnly"] != true {
		return nil, fmt.Errorf("LoginResponse device readOnly changed")
	}
	delete(branch, "readOnly")
	if err := hoistSchemas(doc, schemas); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

func allowSkippedValidation(schemas map[string]any, name string) error {
	risk, err := objectAt(schemas, name, "allOf", 1)
	if err != nil {
		return err
	}
	required, ok := risk["required"].([]any)
	if !ok {
		return fmt.Errorf("%s required list changed", name)
	}
	var kept []any
	found := map[string]bool{}
	for _, v := range required {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s required member is not a string", name)
		}
		if s == "context" || s == "request_token" {
			found[s] = true
		} else {
			kept = append(kept, v)
		}
	}
	if !found["context"] || !found["request_token"] {
		return fmt.Errorf("%s required fields changed", name)
	}
	if len(kept) == 0 {
		delete(risk, "required")
	} else {
		risk["required"] = kept
	}
	return nil
}

func verifySnapshot(snapshot, metadata []byte) error {
	var source struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(metadata, &source); err != nil {
		return fmt.Errorf("parse source metadata: %w", err)
	}
	sum := sha256.Sum256(snapshot)
	if source.SHA256 != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("OpenAPI snapshot checksum does not match source metadata")
	}
	return nil
}

func objectAt(value any, path ...any) (map[string]any, error) {
	for _, part := range path {
		switch key := part.(type) {
		case string:
			object, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected schema object at %v", path)
			}
			value = object[key]
		case int:
			array, ok := value.([]any)
			if !ok || key < 0 || key >= len(array) {
				return nil, fmt.Errorf("expected schema array at %v", path)
			}
			value = array[key]
		default:
			return nil, fmt.Errorf("invalid schema path %v", path)
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected schema object at %v", path)
	}
	return object, nil
}
