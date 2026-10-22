package diff

import (
	"testing"

	"github.com/mateoosoriodelhonte/schemaguard/internal/schema"
)

type want struct {
	sev  Severity
	kind string
	path string
}

func mustParse(t *testing.T, s string) *schema.Schema {
	t.Helper()
	parsed, err := schema.Parse([]byte(s))
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return parsed
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want []want
	}{
		{
			name: "identical",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			want: nil,
		},
		{
			name: "annotation-only change",
			old:  `{"type": "object", "properties": {"a": {"type": "string", "description": "old"}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string", "description": "new"}}}`,
			want: nil,
		},
		{
			name: "optional property added",
			old:  `{"type": "object", "properties": {}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			want: []want{{NonBreaking, KindOptionalPropertyAdded, "a"}},
		},
		{
			name: "required property added",
			old:  `{"type": "object"}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			want: []want{{NonBreaking, KindRequiredPropertyAdded, "a"}},
		},
		{
			name: "optional property removed is breaking",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			new:  `{"type": "object"}`,
			want: []want{{Breaking, KindOptionalPropertyRemoved, "a"}},
		},
		{
			name: "required property removed",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			new:  `{"type": "object"}`,
			want: []want{{Breaking, KindRequiredPropertyRemoved, "a"}},
		},
		{
			name: "optional becomes required",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			want: []want{{NonBreaking, KindPropertyNowRequired, "a"}},
		},
		{
			name: "required becomes optional",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			want: []want{{Breaking, KindPropertyNowOptional, "a"}},
		},
		{
			name: "primitive type changed",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "boolean"}}}`,
			want: []want{{Breaking, KindTypeChanged, "a"}},
		},
		{
			name: "integer widened to number",
			old:  `{"type": "object", "properties": {"a": {"type": "integer"}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "number"}}}`,
			want: []want{{Breaking, KindTypeChanged, "a"}},
		},
		{
			name: "number narrowed to integer",
			old:  `{"type": "object", "properties": {"a": {"type": "number"}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "integer"}}}`,
			want: []want{{NonBreaking, KindTypeNarrowed, "a"}},
		},
		{
			name: "root type changed",
			old:  `{"type": "object"}`,
			new:  `{"type": "string"}`,
			want: []want{{Breaking, KindTypeChanged, "(root)"}},
		},
		{
			name: "object replaced by primitive does not recurse",
			old:  `{"type": "object", "properties": {"a": {"type": "object", "properties": {"b": {"type": "string"}}}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			want: []want{{Breaking, KindTypeChanged, "a"}},
		},
		{
			name: "removed nested object reported once",
			old:  `{"type": "object", "properties": {"a": {"type": "object", "properties": {"b": {"type": "string"}}}}}`,
			new:  `{"type": "object"}`,
			want: []want{{Breaking, KindOptionalPropertyRemoved, "a"}},
		},
		{
			name: "deeply nested changes",
			old: `{"type": "object", "properties": {"a": {"type": "object", "properties": {
				"b": {"type": "object", "properties": {"c": {"type": "string"}, "d": {"type": "integer"}}, "required": ["c"]}}}}}`,
			new: `{"type": "object", "properties": {"a": {"type": "object", "properties": {
				"b": {"type": "object", "properties": {"d": {"type": "string"}, "e": {"type": "null"}}}}}}}`,
			want: []want{
				{Breaking, KindRequiredPropertyRemoved, "a.b.c"},
				{Breaking, KindTypeChanged, "a.b.d"},
				{NonBreaking, KindOptionalPropertyAdded, "a.b.e"},
			},
		},
		{
			name: "requiredness and type change on same property",
			old:  `{"type": "object", "properties": {"a": {"type": "string"}}, "required": ["a"]}`,
			new:  `{"type": "object", "properties": {"a": {"type": "integer"}}}`,
			want: []want{
				{Breaking, KindPropertyNowOptional, "a"},
				{Breaking, KindTypeChanged, "a"},
			},
		},
		{
			name: "breaking findings sorted first",
			old:  `{"type": "object", "properties": {"b": {"type": "string"}}}`,
			new:  `{"type": "object", "properties": {"a": {"type": "string"}}}`,
			want: []want{
				{Breaking, KindOptionalPropertyRemoved, "b"},
				{NonBreaking, KindOptionalPropertyAdded, "a"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(mustParse(t, tt.old), mustParse(t, tt.new))
			if len(got) != len(tt.want) {
				t.Fatalf("got %d findings, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				g := got[i]
				if g.Severity != w.sev || g.Kind != w.kind || g.Path != w.path {
					t.Errorf("finding %d = {%s %s %s}, want {%s %s %s}", i, g.Severity, g.Kind, g.Path, w.sev, w.kind, w.path)
				}
			}
			wantBreaking := false
			for _, w := range tt.want {
				wantBreaking = wantBreaking || w.sev == Breaking
			}
			if HasBreaking(got) != wantBreaking {
				t.Errorf("HasBreaking = %v, want %v", HasBreaking(got), wantBreaking)
			}
		})
	}
}

func TestComparePointer(t *testing.T) {
	old := mustParse(t, `{"type": "object", "properties": {"a/b": {"type": "object", "properties": {"c~d": {"type": "string"}}}}}`)
	new := mustParse(t, `{"type": "object", "properties": {"a/b": {"type": "object"}}}`)
	got := Compare(old, new)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if want := "/properties/a~1b/properties/c~0d"; got[0].Pointer != want {
		t.Errorf("pointer = %q, want %q", got[0].Pointer, want)
	}
	if got[0].Old != "string" {
		t.Errorf("old = %q, want string", got[0].Old)
	}
}
