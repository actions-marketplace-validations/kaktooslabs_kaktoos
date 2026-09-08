package assertion

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/PaesslerAG/jsonpath"
)

// Evaluate runs every assertion in spec against resp and returns one Result
// per assertion. It never returns early: all assertions are evaluated.
// resp.Body is read and closed once.
func Evaluate(resp *http.Response, spec Spec) []Result {
	results := []Result{}

	if spec.Status != nil {
		results = append(results, Result{
			Type:     "status",
			Expected: *spec.Status,
			Actual:   resp.StatusCode,
			Passed:   resp.StatusCode == *spec.Status,
		})
	}

	if len(spec.Body) == 0 {
		return results
	}

	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()

	var data interface{}
	parseErr := readErr
	if parseErr == nil {
		parseErr = json.Unmarshal(body, &data)
	}

	for _, ba := range spec.Body {
		r := Result{Path: ba.Path}
		switch {
		case ba.Exists:
			r.Type = "exists"
		case ba.NotExists:
			r.Type = "not_exists"
		default:
			r.Type = "equals"
			r.Expected = ba.Equals
		}

		if parseErr != nil {
			r.Error = fmt.Sprintf("response body is not valid JSON: %v", parseErr)
			results = append(results, r)
			continue
		}

		val, err := jsonpath.Get(ba.Path, data)
		matched := err == nil

		switch r.Type {
		case "exists":
			r.Passed = matched
			if matched {
				r.Actual = val
			} else {
				r.Error = fmt.Sprintf("no match for %s", ba.Path)
			}
		case "not_exists":
			r.Passed = !matched
			if matched {
				r.Actual = val
			}
		case "equals":
			if !matched {
				r.Error = fmt.Sprintf("no match for %s: %v", ba.Path, err)
			} else {
				r.Actual = val
				r.Passed = reflect.DeepEqual(normaliseNumeric(ba.Equals), normaliseNumeric(val))
			}
		}
		results = append(results, r)
	}
	return results
}

// normaliseNumeric converts every integer-typed value (YAML decodes to int,
// JSON decodes to float64) into float64 so DeepEqual compares numbers by value.
// Maps and slices are normalised recursively.
func normaliseNumeric(v interface{}) interface{} {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case float32:
		return float64(x)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, e := range x {
			out[k] = normaliseNumeric(e)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, e := range x {
			out[i] = normaliseNumeric(e)
		}
		return out
	default:
		return v
	}
}
