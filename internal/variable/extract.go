package variable

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/PaesslerAG/jsonpath"
)

// coerceToString converts a JSONPath result into its string representation:
// strings pass through, bools become "true"/"false", whole floats lose the
// decimal point, nil becomes "null", objects/arrays become compact JSON.
func coerceToString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		// objects and arrays: compact JSON
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

// Extract unmarshals the JSON response body, evaluates each JSONPath
// expression in exprs (variable name → expression), and stores the coerced
// string result in the store, overwriting existing entries.
// It returns an error on a non-JSON body or when an expression has no match.
func Extract(jsonBody []byte, exprs map[string]string, s *Store, stepName string) error {
	var data interface{}
	if err := json.Unmarshal(jsonBody, &data); err != nil {
		return fmt.Errorf("step %s: response body is not valid JSON: %w", stepName, err)
	}
	for name, expr := range exprs {
		val, err := jsonpath.Get(expr, data)
		if err != nil {
			return fmt.Errorf("step %s: extract %s: no match for expression %s: %w", stepName, name, expr, err)
		}
		s.Set(name, coerceToString(val))
	}
	return nil
}
