package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr string
	}{
		{
			name:     "nested breaking changes",
			args:     []string{"testdata/nested/old.json", "testdata/nested/new.json"},
			wantCode: exitBreaking,
			wantStdout: []string{
				"BREAKING (5)",
				"email [property-now-optional]",
				"id [type-changed]",
				"nickname [optional-property-removed]",
				"profile.address.postalCode [required-property-removed]",
				"score [type-changed]",
				"NON-BREAKING (3)",
				"createdAt [optional-property-added]",
				"profile.address.country [required-property-added]",
				"profile.age [property-now-required]",
				"Result: BREAKING (5 breaking, 3 non-breaking)",
			},
		},
		{
			name:       "compatible changes",
			args:       []string{"testdata/compatible/old.json", "testdata/compatible/new.json"},
			wantCode:   exitOK,
			wantStdout: []string{"NON-BREAKING (3)", "Result: OK (0 breaking, 3 non-breaking)"},
		},
		{
			name:       "identical files",
			args:       []string{"testdata/nested/old.json", "testdata/nested/old.json"},
			wantCode:   exitOK,
			wantStdout: []string{"No changes detected.", "Result: OK"},
		},
		{
			name:       "flag after files",
			args:       []string{"testdata/nested/old.json", "testdata/nested/new.json", "--json"},
			wantCode:   exitBreaking,
			wantStdout: []string{`"breaking": true`},
		},
		{
			name:       "unsupported schema",
			args:       []string{"testdata/unsupported/ref.json", "testdata/nested/new.json"},
			wantCode:   exitError,
			wantStderr: `unsupported keyword "$defs"`,
		},
		{
			name:       "malformed JSON",
			args:       []string{"testdata/nested/old.json", "testdata/invalid/malformed.json"},
			wantCode:   exitError,
			wantStderr: "invalid JSON",
		},
		{
			name:       "missing file",
			args:       []string{"testdata/does-not-exist.json", "testdata/nested/new.json"},
			wantCode:   exitError,
			wantStderr: "does-not-exist.json",
		},
		{
			name:       "too few arguments",
			args:       []string{"testdata/nested/old.json"},
			wantCode:   exitError,
			wantStderr: "expected 2 schema files, got 1",
		},
		{
			name:       "unknown option",
			args:       []string{"--yaml", "a.json", "b.json"},
			wantCode:   exitError,
			wantStderr: `unknown option "--yaml"`,
		},
		{
			name:       "help",
			args:       []string{"--help"},
			wantCode:   exitOK,
			wantStdout: []string{"Usage: schemaguard"},
		},
		{
			name:       "version",
			args:       []string{"--version"},
			wantCode:   exitOK,
			wantStdout: []string{"schemaguard " + version},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s", code, tt.wantCode, stdout.String(), stderr.String())
			}
			for _, s := range tt.wantStdout {
				if !strings.Contains(stdout.String(), s) {
					t.Errorf("stdout missing %q:\n%s", s, stdout.String())
				}
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr missing %q:\n%s", tt.wantStderr, stderr.String())
			}
		})
	}
}

func TestRunJSON(t *testing.T) {
	tests := []struct {
		name         string
		old, new     string
		wantCode     int
		wantBreaking bool
		wantCounts   jsonSummary
	}{
		{"nested", "testdata/nested/old.json", "testdata/nested/new.json", exitBreaking, true, jsonSummary{5, 3}},
		{"compatible", "testdata/compatible/old.json", "testdata/compatible/new.json", exitOK, false, jsonSummary{0, 3}},
		{"identical", "testdata/compatible/old.json", "testdata/compatible/old.json", exitOK, false, jsonSummary{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"--json", tt.old, tt.new}, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			var rep jsonReport
			if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
				t.Fatalf("invalid JSON output: %v\n%s", err, stdout.String())
			}
			if rep.Breaking != tt.wantBreaking || rep.Summary != tt.wantCounts {
				t.Errorf("got breaking=%v summary=%+v", rep.Breaking, rep.Summary)
			}
			if rep.Findings == nil || len(rep.Findings) != tt.wantCounts.Breaking+tt.wantCounts.NonBreaking {
				t.Errorf("findings = %+v", rep.Findings)
			}
			if !strings.Contains(stdout.String(), `"findings": [`) {
				t.Errorf("findings must be encoded as an array:\n%s", stdout.String())
			}
		})
	}
}
