// Package schema validates HTTP responses against the response schemas
// declared in an OpenAPI operation. It wraps kin-openapi's own JSON Schema
// validator (openapi3.Schema.VisitJSON) rather than reimplementing one.
package schema

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
)

// Mode controls whether/how schema violations affect step pass/fail.
// Validate itself is mode-independent: it always returns the full violation
// list, so warn and strict report identical violations — only the caller's
// pass/fail decision differs.
type Mode string

const (
	Off    Mode = "off"
	Warn   Mode = "warn"
	Strict Mode = "strict"
)

// Violation kinds.
const (
	KindStatusUndeclared     = "status_undeclared"
	KindContentTypeMismatch  = "content_type_mismatch"
	KindRequiredFieldMissing = "required_field_missing"
	KindSchemaMismatch       = "schema_mismatch"
)

type Violation struct {
	Kind    string
	Path    string // JSON pointer, e.g. "$.id"; empty for status/content-type kinds
	Message string
}

type Result struct {
	Violations       []Violation
	UndeclaredFields []string // informational only, never causes failure
}

// Validate checks an HTTP response against the response declared for
// statusCode on op. It performs no I/O and never fails the caller directly;
// the caller decides what a Mode does with the returned violations.
func Validate(op openapi.Operation, statusCode int, contentType string, body []byte) Result {
	resp := op.Responses[strconv.Itoa(statusCode)]
	if resp == nil {
		resp = op.Responses["default"]
	}
	if resp == nil {
		return Result{Violations: []Violation{{
			Kind:    KindStatusUndeclared,
			Message: fmt.Sprintf("status %d is not declared for this operation", statusCode),
		}}}
	}
	if len(resp.Content) == 0 {
		// No content declared for this response (e.g. 204) — nothing to check.
		return Result{}
	}

	mt := stripMediaTypeParams(contentType)
	mediaType := resp.Content[mt]
	if mediaType == nil {
		return Result{Violations: []Violation{{
			Kind:    KindContentTypeMismatch,
			Message: fmt.Sprintf("response content-type %q is not declared for status %d", contentType, statusCode),
		}}}
	}
	if mediaType.Schema == nil || mediaType.Schema.Value == nil {
		return Result{}
	}
	sv := mediaType.Schema.Value

	if len(body) == 0 {
		return Result{}
	}
	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Result{Violations: []Violation{{
			Kind:    KindSchemaMismatch,
			Message: fmt.Sprintf("response body is not valid JSON: %v", err),
		}}}
	}

	var violations []Violation
	if err := sv.VisitJSON(parsed, openapi3.MultiErrors()); err != nil {
		violations = append(violations, toViolations(err)...)
	}

	return Result{
		Violations:       violations,
		UndeclaredFields: undeclaredFields(sv, parsed),
	}
}

func stripMediaTypeParams(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}

func toViolations(err error) []Violation {
	var out []Violation
	var walk func(error)
	walk = func(e error) {
		if me, ok := e.(openapi3.MultiError); ok {
			for _, sub := range me {
				walk(sub)
			}
			return
		}
		if se, ok := e.(*openapi3.SchemaError); ok {
			path := "$"
			if p := se.JSONPointer(); len(p) > 0 {
				path = "$." + strings.Join(p, ".")
			}
			kind := KindSchemaMismatch
			if se.SchemaField == "required" {
				kind = KindRequiredFieldMissing
			}
			out = append(out, Violation{Kind: kind, Path: path, Message: se.Reason})
			return
		}
		out = append(out, Violation{Kind: KindSchemaMismatch, Message: e.Error()})
	}
	walk(err)
	return out
}

// undeclaredFields reports top-level and nested object keys present in the
// actual response but not declared in the schema's properties. It is
// informational only. A schema that explicitly allows additional properties
// (additionalProperties: true) suppresses detection at that level entirely.
// Ponytail: no $ref-cycle / allOf / oneOf composition handling here — add if
// a real spec surfaces undeclared-field noise from a composed schema.
func undeclaredFields(sv *openapi3.Schema, value interface{}) []string {
	var out []string
	var walk func(sv *openapi3.Schema, v interface{}, prefix string)
	walk = func(sv *openapi3.Schema, v interface{}, prefix string) {
		if sv == nil {
			return
		}
		switch val := v.(type) {
		case map[string]interface{}:
			if len(sv.Properties) == 0 {
				return
			}
			if sv.AdditionalProperties.Has != nil && *sv.AdditionalProperties.Has {
				return
			}
			for k, vv := range val {
				propSchema, declared := sv.Properties[k]
				path := k
				if prefix != "" {
					path = prefix + "." + k
				}
				if !declared {
					out = append(out, "$."+path)
					continue
				}
				if propSchema != nil && propSchema.Value != nil {
					walk(propSchema.Value, vv, path)
				}
			}
		case []interface{}:
			if sv.Items != nil && sv.Items.Value != nil {
				for _, item := range val {
					walk(sv.Items.Value, item, prefix)
				}
			}
		}
	}
	walk(sv, value, "")
	return out
}
