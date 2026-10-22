package schema

import (
	"errors"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, s *Schema)
	}{
		{
			name:  "primitive",
			input: `{"type": "string"}`,
			check: func(t *testing.T, s *Schema) {
				if s.Type != "string" || s.Properties != nil || s.Required != nil {
					t.Errorf("got %+v", s)
				}
			},
		},
		{
			name:  "empty object",
			input: `{"type": "object"}`,
			check: func(t *testing.T, s *Schema) {
				if len(s.Properties) != 0 || len(s.Required) != 0 || s.Properties == nil {
					t.Errorf("got %+v", s)
				}
			},
		},
		{
			name: "nested object with required",
			input: `{"type": "object",
				"properties": {"a": {"type": "object",
					"properties": {"b": {"type": "integer"}},
					"required": ["b"]}},
				"required": ["a"]}`,
			check: func(t *testing.T, s *Schema) {
				a := s.Properties["a"]
				if a == nil || !s.Required["a"] || a.Properties["b"].Type != "integer" || !a.Required["b"] {
					t.Errorf("nested structure not parsed: %+v", s)
				}
			},
		},
		{
			name: "annotations ignored",
			input: `{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "x",
				"title": "T", "description": "D", "$comment": "c", "examples": [1],
				"default": null, "deprecated": false, "type": "null"}`,
			check: func(t *testing.T, s *Schema) {
				if s.Type != "null" {
					t.Errorf("got type %q", s.Type)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, s)
		})
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty input", ``, "unexpected end of input"},
		{"malformed JSON", `{"type": `, "invalid JSON"},
		{"trailing data", `{"type": "string"} {}`, "unexpected data after top-level value"},
		{"duplicate key", `{"type": "string", "type": "integer"}`, `duplicate object key "type"`},
		{"nested duplicate key", `{"type": "object", "properties": {"a": {"type": "string"}, "a": {"type": "string"}}}`, `duplicate object key "a"`},
		{"non-object root", `[]`, "schema must be a JSON object"},
		{"boolean schema", `{"type": "object", "properties": {"a": true}}`, "#/properties/a: boolean schemas are not supported"},
		{"missing type", `{"properties": {}}`, `missing "type"`},
		{"type array", `{"type": ["string", "null"]}`, "type arrays"},
		{"type not string", `{"type": 1}`, "type must be a string"},
		{"array type", `{"type": "array"}`, `type "array" is not supported`},
		{"unknown type", `{"type": "date"}`, `unknown type "date"`},
		{"$ref", `{"$ref": "#/$defs/x"}`, `unsupported keyword "$ref"`},
		{"items", `{"type": "object", "properties": {"a": {"type": "string", "items": {}}}}`, `#/properties/a: unsupported keyword "items"`},
		{"oneOf", `{"type": "object", "oneOf": []}`, `unsupported keyword "oneOf"`},
		{"additionalProperties", `{"type": "object", "additionalProperties": false}`, `unsupported keyword "additionalProperties"`},
		{"enum", `{"type": "string", "enum": ["a"]}`, `unsupported keyword "enum"`},
		{"format", `{"type": "string", "format": "date-time"}`, `unsupported keyword "format"`},
		{"properties on primitive", `{"type": "string", "properties": {}}`, `"properties" is only allowed when type is "object"`},
		{"required on primitive", `{"type": "string", "required": []}`, `"required" is only allowed when type is "object"`},
		{"properties not object", `{"type": "object", "properties": []}`, "properties must be an object"},
		{"required not array", `{"type": "object", "required": "a"}`, "required must be an array"},
		{"required non-string", `{"type": "object", "properties": {"a": {"type": "string"}}, "required": [1]}`, "#/required/0: required entries must be strings"},
		{"duplicate required", `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a", "a"]}`, `#/required/1: duplicate required entry "a"`},
		{"undeclared required", `{"type": "object", "properties": {}, "required": ["a"]}`, `required property "a" is not declared`},
		{"escaped pointer", `{"type": "object", "properties": {"a/b": {"type": "x"}}}`, "#/properties/a~1b/type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaErrorType(t *testing.T) {
	_, err := Parse([]byte(`{"type": "object", "properties": {"a": {"type": "array"}}}`))
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if se.Pointer != "/properties/a/type" {
		t.Errorf("pointer = %q", se.Pointer)
	}
}
