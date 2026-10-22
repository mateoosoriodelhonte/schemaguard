// Command schemaguard compares two JSON Schema files and reports potentially
// breaking API contract changes.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mateoosoriodelhonte/schemaguard/internal/diff"
	"github.com/mateoosoriodelhonte/schemaguard/internal/schema"
)

const version = "0.1.0"

const (
	exitOK       = 0
	exitBreaking = 1
	exitError    = 2
)

const usage = `Usage: schemaguard [--json] <old.json> <new.json>

Compares two JSON Schema files under a response-schema compatibility model
and reports breaking and non-breaking changes.

Options:
  --json       Emit machine-readable JSON instead of text
  -h, --help   Show this help
  --version    Print the version

Exit codes:
  0  no breaking changes
  1  breaking changes found
  2  invalid input or unsupported schema
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var jsonOut, endOfFlags bool
	var files []string
	for _, a := range args {
		if endOfFlags || !strings.HasPrefix(a, "-") || a == "-" {
			files = append(files, a)
			continue
		}
		switch a {
		case "--":
			endOfFlags = true
		case "--json", "-json":
			jsonOut = true
		case "-h", "--help", "-help":
			fmt.Fprint(stdout, usage)
			return exitOK
		case "--version", "-version":
			fmt.Fprintln(stdout, "schemaguard", version)
			return exitOK
		default:
			fmt.Fprintf(stderr, "schemaguard: unknown option %q\n\n%s", a, usage)
			return exitError
		}
	}
	if len(files) != 2 {
		fmt.Fprintf(stderr, "schemaguard: expected 2 schema files, got %d\n\n%s", len(files), usage)
		return exitError
	}

	oldS, err := load(files[0])
	if err != nil {
		fmt.Fprintf(stderr, "schemaguard: %v\n", err)
		return exitError
	}
	newS, err := load(files[1])
	if err != nil {
		fmt.Fprintf(stderr, "schemaguard: %v\n", err)
		return exitError
	}

	findings := diff.Compare(oldS, newS)
	if jsonOut {
		if err := writeJSON(stdout, files[0], files[1], findings); err != nil {
			fmt.Fprintf(stderr, "schemaguard: writing output: %v\n", err)
			return exitError
		}
	} else {
		writeText(stdout, files[0], files[1], findings)
	}

	if diff.HasBreaking(findings) {
		return exitBreaking
	}
	return exitOK
}

func load(path string) (*schema.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := schema.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

func countBreaking(findings []diff.Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == diff.Breaking {
			n++
		}
	}
	return n
}

func writeText(w io.Writer, oldPath, newPath string, findings []diff.Finding) {
	fmt.Fprintf(w, "Comparing %s -> %s\n\n", oldPath, newPath)
	if len(findings) == 0 {
		fmt.Fprint(w, "No changes detected.\n\nResult: OK\n")
		return
	}

	breaking := countBreaking(findings)
	section := func(title string, sev diff.Severity, count int) {
		if count == 0 {
			return
		}
		fmt.Fprintf(w, "%s (%d)\n", title, count)
		for _, f := range findings {
			if f.Severity == sev {
				fmt.Fprintf(w, "  - %s [%s]\n      %s\n", f.Path, f.Kind, f.Message)
			}
		}
		fmt.Fprintln(w)
	}
	section("BREAKING", diff.Breaking, breaking)
	section("NON-BREAKING", diff.NonBreaking, len(findings)-breaking)

	result := "OK"
	if breaking > 0 {
		result = "BREAKING"
	}
	fmt.Fprintf(w, "Result: %s (%d breaking, %d non-breaking)\n", result, breaking, len(findings)-breaking)
}

type jsonSummary struct {
	Breaking    int `json:"breaking"`
	NonBreaking int `json:"nonBreaking"`
}

type jsonReport struct {
	Old      string         `json:"old"`
	New      string         `json:"new"`
	Breaking bool           `json:"breaking"`
	Summary  jsonSummary    `json:"summary"`
	Findings []diff.Finding `json:"findings"`
}

func writeJSON(w io.Writer, oldPath, newPath string, findings []diff.Finding) error {
	breaking := countBreaking(findings)
	if findings == nil {
		findings = []diff.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonReport{
		Old:      oldPath,
		New:      newPath,
		Breaking: breaking > 0,
		Summary:  jsonSummary{Breaking: breaking, NonBreaking: len(findings) - breaking},
		Findings: findings,
	})
}
