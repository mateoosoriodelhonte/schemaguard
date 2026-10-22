# SchemaGuard

[![CI](https://github.com/mateoosoriodelhonte/schemaguard/actions/workflows/ci.yml/badge.svg)](https://github.com/mateoosoriodelhonte/schemaguard/actions/workflows/ci.yml)

SchemaGuard compares two JSON Schema files and reports API contract changes
that could break consumers. It is a single static binary built with the Go
standard library only.

SchemaGuard deliberately supports a **small, documented subset** of JSON
Schema and rejects everything else. It does not aim for full JSON Schema
compatibility.

## Install

```bash
go install github.com/mateoosoriodelhonte/schemaguard@latest
```

Or build from source with `go build .`.

## Usage

```
schemaguard [--json] <old.json> <new.json>
```

| Exit code | Meaning                                        |
|-----------|------------------------------------------------|
| `0`       | No breaking changes (non-breaking ones may exist) |
| `1`       | At least one breaking change                   |
| `2`       | Invalid arguments, invalid JSON, or unsupported schema |

Use it as a CI gate: `schemaguard api/v1.json api/v2.json || exit 1`.

### Example

```
$ schemaguard testdata/nested/old.json testdata/nested/new.json
Comparing testdata/nested/old.json -> testdata/nested/new.json

BREAKING (5)
  - email [property-now-optional]
      property is no longer required; consumers may receive responses without it
  - id [type-changed]
      type changed from integer to string
  - nickname [optional-property-removed]
      optional property was removed; consumers that read it when present will no longer receive it
  - profile.address.postalCode [required-property-removed]
      required property was removed
  - score [type-changed]
      type widened from integer to number; consumers may receive non-integer values

NON-BREAKING (3)
  - createdAt [optional-property-added]
      optional property was added; consumers that ignore unknown fields are unaffected
  - profile.address.country [required-property-added]
      required property was added; consumers that ignore unknown fields are unaffected
  - profile.age [property-now-required]
      property is now required; it is always present for consumers

Result: BREAKING (5 breaking, 3 non-breaking)
```

### JSON output

`--json` writes a machine-readable report to stdout. Errors always go to
stderr as text with exit code 2.

```json
{
  "old": "testdata/compatible/old.json",
  "new": "testdata/compatible/new.json",
  "breaking": false,
  "summary": { "breaking": 0, "nonBreaking": 3 },
  "findings": [
    {
      "severity": "NON-BREAKING",
      "kind": "optional-property-added",
      "path": "meta.region",
      "pointer": "/properties/meta/properties/region",
      "message": "optional property was added; consumers that ignore unknown fields are unaffected",
      "new": "string"
    }
  ]
}
```

`pointer` is an RFC 6901 JSON Pointer into the schema and is unambiguous even
when property names contain dots. `old`/`new` on a finding hold the previous
and new type or requiredness where relevant.

## Compatibility model

SchemaGuard treats every schema as a **response schema**: it describes data an
API *sends*, and consumers were written against the old schema. A change is
**BREAKING** if a consumer relying on the old contract could fail or silently
lose data.

Assumptions:

1. Consumers **ignore unknown fields**. Adding properties is therefore safe.
2. Consumers **may read any documented property**, required or optional.
   Removing a property is therefore breaking even when it was optional: a
   consumer that used it whenever it was present loses that data.
3. Consumers **rely on required properties being present** and on declared
   types being accurate.

| Change                                   | Kind                        | Severity     |
|------------------------------------------|-----------------------------|--------------|
| Required property removed                | `required-property-removed` | BREAKING     |
| Optional property removed                | `optional-property-removed` | BREAKING     |
| Required property becomes optional       | `property-now-optional`     | BREAKING     |
| Type changed (e.g. `string` → `integer`) | `type-changed`              | BREAKING     |
| `integer` widened to `number`            | `type-changed`              | BREAKING     |
| `number` narrowed to `integer`           | `type-narrowed`             | NON-BREAKING |
| Optional property added                  | `optional-property-added`   | NON-BREAKING |
| Required property added                  | `required-property-added`   | NON-BREAKING |
| Optional property becomes required       | `property-now-required`     | NON-BREAKING |

Note the asymmetry: **adding** an optional field is non-breaking, while
**removing** one is breaking.

Nested objects are compared recursively. When a property is removed or its
type changes, its children are not reported separately. Requiredness is taken
from each object's `required` array, not inferred from property presence.

For **request** schemas (data clients send) most of these rules invert; for
example, a newly required request field breaks existing clients. SchemaGuard
does not implement a request model.

## Supported JSON Schema subset

Every schema node must be a JSON object with a single string `type`:

- `type`: `object`, `string`, `number`, `integer`, `boolean`, or `null`
- `properties` (objects only): map of property name to schema
- `required` (objects only): array of unique strings, each declared in `properties`
- Annotations, accepted and ignored: `$schema`, `$id`, `$comment`, `title`,
  `description`, `examples`, `default`, `deprecated`

Anything else causes exit code 2 with the JSON Pointer of the offending
location, including:

- `$ref`, `$defs`, `allOf`/`anyOf`/`oneOf`/`not`, `if`/`then`/`else`
- `type: "array"`, `items`, type arrays such as `["string", "null"]`
- `additionalProperties`, `patternProperties`, `enum`, `const`, `format`,
  and all numeric/string/object validation keywords
- Boolean schemas, missing `type`, `required` names not in `properties`
- Malformed JSON, duplicate object keys, and trailing data

```
$ schemaguard testdata/unsupported/ref.json testdata/nested/new.json
schemaguard: testdata/unsupported/ref.json: #: unsupported keyword "$defs"
```

## Limitations

- Only the subset above is supported; real-world schemas using `$ref`,
  arrays, `enum`, or composition keywords must be simplified first.
- Only response-schema semantics are implemented.
- `additionalProperties` is not modeled; consumers are assumed to tolerate
  unknown fields.
- Annotation changes (e.g. `description`, `deprecated`) are not reported.

## Development

```bash
go test ./...
go vet ./...
go build ./...
```

Fixtures live in [`testdata/`](testdata). CI runs gofmt, vet, race-enabled
tests, and a build on Go 1.22 and the latest stable release.

## License

[MIT](LICENSE)
