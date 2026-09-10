package webhook

import (
	"fmt"
	"strings"
)

// ExtractVariables resolves variable_mapping entries against the webhook request.
// pathParams are URL path parameters captured by matchPath; body is the parsed
// JSON body. Returns extracted values as strings, or an error naming the missing
// source if any mapping cannot be resolved.
func ExtractVariables(
	mapping map[string]string,
	pathParams map[string]string,
	body map[string]interface{},
) (map[string]string, error) {
	result := make(map[string]string, len(mapping))

	for name, source := range mapping {
		switch {
		case strings.HasPrefix(source, "path."):
			key := strings.TrimPrefix(source, "path.")
			val, ok := pathParams[key]
			if !ok {
				return nil, fmt.Errorf("webhook variable %q: path parameter %q not found", name, key)
			}
			result[name] = val

		case strings.HasPrefix(source, "body."):
			path := strings.TrimPrefix(source, "body.")
			val, err := lookupBody(body, strings.Split(path, "."))
			if err != nil {
				return nil, fmt.Errorf("webhook variable %q: %w", name, err)
			}
			result[name] = fmt.Sprintf("%v", val)

		default:
			return nil, fmt.Errorf("webhook variable %q: unsupported mapping source %q", name, source)
		}
	}

	return result, nil
}

func lookupBody(body map[string]interface{}, fields []string) (interface{}, error) {
	var cur interface{} = body
	for i, f := range fields {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("body field %q is not an object", strings.Join(fields[:i], "."))
		}
		v, ok := m[f]
		if !ok {
			return nil, fmt.Errorf("body field %q not found", strings.Join(fields[:i+1], "."))
		}
		cur = v
	}
	return cur, nil
}
