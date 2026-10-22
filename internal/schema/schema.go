// Package schema parses the subset of JSON Schema that SchemaGuard supports.
//
// The supported subset is intentionally small: every schema node must declare
// a single "type" (object, string, number, integer, boolean or null), and
// object nodes may declare "properties" and "required". Annotation keywords
// that do not affect validation are accepted and ignored. Every other keyword
// is rejected so that unsupported constructs are never silently misclassified.
package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Schema is a parsed node of the supported JSON Schema subset.
type Schema struct {
	// Type is one of "object", "string", "number", "integer", "boolean", "null".
	Type string
	// Properties holds child schemas. It is non-nil only for objects.
	Properties map[string]*Schema
	// Required is the set of required property names. It is non-nil only for objects.
	Required map[string]bool
}

// Error reports an invalid or unsupported schema construct at a location.
type Error struct {
	// Pointer is the JSON Pointer (RFC 6901) of the offending location.
	Pointer string
	Msg     string
}

func (e *Error) Error() string {
	return fmt.Sprintf("#%s: %s", e.Pointer, e.Msg)
}

func errorf(ptr, format string, args ...any) error {
	return &Error{Pointer: ptr, Msg: fmt.Sprintf(format, args...)}
}

var supportedTypes = map[string]bool{
	"object":  true,
	"string":  true,
	"number":  true,
	"integer": true,
	"boolean": true,
	"null":    true,
}

// annotationKeywords do not affect which instances validate, so they are
// accepted and ignored.
var annotationKeywords = map[string]bool{
	"$schema":     true,
	"$id":         true,
	"$comment":    true,
	"title":       true,
	"description": true,
	"examples":    true,
	"default":     true,
	"deprecated":  true,
}

// Parse decodes data as JSON and validates it against the supported subset.
func Parse(data []byte) (*Schema, error) {
	v, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return parseNode(v, "")
}

// EscapePointer escapes a single JSON Pointer reference token.
func EscapePointer(token string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(token)
}

func parseNode(v any, ptr string) (*Schema, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		if _, isBool := v.(bool); isBool {
			return nil, errorf(ptr, "boolean schemas are not supported")
		}
		return nil, errorf(ptr, "schema must be a JSON object")
	}

	for _, k := range sortedKeys(obj) {
		if k == "type" || k == "properties" || k == "required" || annotationKeywords[k] {
			continue
		}
		return nil, errorf(ptr, "unsupported keyword %q", k)
	}

	rawType, ok := obj["type"]
	if !ok {
		return nil, errorf(ptr, `missing "type": every schema node must declare a single type`)
	}
	typ, ok := rawType.(string)
	if !ok {
		if _, isList := rawType.([]any); isList {
			return nil, errorf(ptr+"/type", "multiple types (type arrays) are not supported")
		}
		return nil, errorf(ptr+"/type", "type must be a string")
	}
	if typ == "array" {
		return nil, errorf(ptr+"/type", `type "array" is not supported`)
	}
	if !supportedTypes[typ] {
		return nil, errorf(ptr+"/type", "unknown type %q", typ)
	}

	s := &Schema{Type: typ}
	if typ != "object" {
		for _, k := range []string{"properties", "required"} {
			if _, has := obj[k]; has {
				return nil, errorf(ptr+"/"+k, `%q is only allowed when type is "object"`, k)
			}
		}
		return s, nil
	}

	s.Properties = map[string]*Schema{}
	s.Required = map[string]bool{}

	if raw, has := obj["properties"]; has {
		props, ok := raw.(map[string]any)
		if !ok {
			return nil, errorf(ptr+"/properties", "properties must be an object")
		}
		for _, name := range sortedKeys(props) {
			child, err := parseNode(props[name], ptr+"/properties/"+EscapePointer(name))
			if err != nil {
				return nil, err
			}
			s.Properties[name] = child
		}
	}

	if raw, has := obj["required"]; has {
		list, ok := raw.([]any)
		if !ok {
			return nil, errorf(ptr+"/required", "required must be an array of strings")
		}
		for i, item := range list {
			itemPtr := fmt.Sprintf("%s/required/%d", ptr, i)
			name, ok := item.(string)
			if !ok {
				return nil, errorf(itemPtr, "required entries must be strings")
			}
			if s.Required[name] {
				return nil, errorf(itemPtr, "duplicate required entry %q", name)
			}
			if _, declared := s.Properties[name]; !declared {
				return nil, errorf(itemPtr, "required property %q is not declared in properties (undeclared required properties are not supported)", name)
			}
			s.Required[name] = true
		}
	}

	return s, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// decodeStrict decodes a single JSON value, rejecting duplicate object keys
// and trailing data, which encoding/json would otherwise accept silently.
func decodeStrict(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, errors.New("unexpected data after top-level value")
		}
		return nil, err
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return nil, errors.New("unexpected end of input")
		}
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch delim {
	case '{':
		obj := map[string]any{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := kt.(string)
			if !ok {
				return nil, fmt.Errorf("invalid object key %v", kt)
			}
			if _, dup := obj[key]; dup {
				return nil, fmt.Errorf("duplicate object key %q", key)
			}
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			obj[key] = val
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return obj, nil
	case '[':
		arr := []any{}
		for dec.More() {
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delim)
	}
}
