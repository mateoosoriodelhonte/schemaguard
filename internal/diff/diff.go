// Package diff compares two schemas under a response-schema compatibility
// model: the schema describes data that a producer (an API) sends, and the
// question is whether consumers written against the old schema can still
// rely on everything it promised.
package diff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mateoosoriodelhonte/schemaguard/internal/schema"
)

// Severity classifies a finding.
type Severity string

const (
	Breaking    Severity = "BREAKING"
	NonBreaking Severity = "NON-BREAKING"
)

// Finding kinds.
const (
	KindRequiredPropertyRemoved = "required-property-removed"
	KindOptionalPropertyRemoved = "optional-property-removed"
	KindRequiredPropertyAdded   = "required-property-added"
	KindOptionalPropertyAdded   = "optional-property-added"
	KindPropertyNowOptional     = "property-now-optional"
	KindPropertyNowRequired     = "property-now-required"
	KindTypeChanged             = "type-changed"
	KindTypeNarrowed            = "type-narrowed"
)

// Finding is a single detected change between two schemas.
type Finding struct {
	Severity Severity `json:"severity"`
	Kind     string   `json:"kind"`
	// Path is a human-readable dotted property path; "(root)" for the root.
	Path string `json:"path"`
	// Pointer is the JSON Pointer of the affected schema node.
	Pointer string `json:"pointer"`
	Message string `json:"message"`
	Old     string `json:"old,omitempty"`
	New     string `json:"new,omitempty"`
}

// Compare returns all findings between oldS and newS, breaking findings
// first, each group in depth-first property-name order.
func Compare(oldS, newS *schema.Schema) []Finding {
	c := &comparer{findings: []Finding{}}
	c.node(oldS, newS, nil, "")
	sort.SliceStable(c.findings, func(i, j int) bool {
		return c.findings[i].Severity == Breaking && c.findings[j].Severity != Breaking
	})
	return c.findings
}

// HasBreaking reports whether any finding is breaking.
func HasBreaking(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == Breaking {
			return true
		}
	}
	return false
}

type comparer struct {
	findings []Finding
}

func (c *comparer) add(sev Severity, kind string, path []string, ptr, oldV, newV, msg string) {
	c.findings = append(c.findings, Finding{
		Severity: sev,
		Kind:     kind,
		Path:     displayPath(path),
		Pointer:  ptr,
		Message:  msg,
		Old:      oldV,
		New:      newV,
	})
}

func (c *comparer) node(o, n *schema.Schema, path []string, ptr string) {
	if o.Type != n.Type {
		switch {
		case o.Type == "number" && n.Type == "integer":
			c.add(NonBreaking, KindTypeNarrowed, path, ptr, o.Type, n.Type,
				"type narrowed from number to integer; every new value is still a valid number")
		case o.Type == "integer" && n.Type == "number":
			c.add(Breaking, KindTypeChanged, path, ptr, o.Type, n.Type,
				"type widened from integer to number; consumers may receive non-integer values")
		default:
			c.add(Breaking, KindTypeChanged, path, ptr, o.Type, n.Type,
				fmt.Sprintf("type changed from %s to %s", o.Type, n.Type))
		}
		// Children of an incompatible type are not compared further.
		return
	}
	if o.Type != "object" {
		return
	}

	for _, name := range unionKeys(o.Properties, n.Properties) {
		op, inOld := o.Properties[name]
		np, inNew := n.Properties[name]
		oReq, nReq := o.Required[name], n.Required[name]
		childPath := append(path[:len(path):len(path)], name)
		childPtr := ptr + "/properties/" + schema.EscapePointer(name)

		switch {
		case inOld && !inNew:
			if oReq {
				c.add(Breaking, KindRequiredPropertyRemoved, childPath, childPtr, op.Type, "",
					"required property was removed")
			} else {
				c.add(Breaking, KindOptionalPropertyRemoved, childPath, childPtr, op.Type, "",
					"optional property was removed; consumers that read it when present will no longer receive it")
			}
		case !inOld && inNew:
			if nReq {
				c.add(NonBreaking, KindRequiredPropertyAdded, childPath, childPtr, "", np.Type,
					"required property was added; consumers that ignore unknown fields are unaffected")
			} else {
				c.add(NonBreaking, KindOptionalPropertyAdded, childPath, childPtr, "", np.Type,
					"optional property was added; consumers that ignore unknown fields are unaffected")
			}
		default:
			if oReq && !nReq {
				c.add(Breaking, KindPropertyNowOptional, childPath, childPtr, "required", "optional",
					"property is no longer required; consumers may receive responses without it")
			} else if !oReq && nReq {
				c.add(NonBreaking, KindPropertyNowRequired, childPath, childPtr, "optional", "required",
					"property is now required; it is always present for consumers")
			}
			c.node(op, np, childPath, childPtr)
		}
	}
}

func unionKeys(a, b map[string]*schema.Schema) []string {
	seen := make(map[string]bool, len(a)+len(b))
	keys := make([]string, 0, len(a)+len(b))
	for _, m := range []map[string]*schema.Schema{a, b} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func displayPath(path []string) string {
	if len(path) == 0 {
		return "(root)"
	}
	return strings.Join(path, ".")
}
