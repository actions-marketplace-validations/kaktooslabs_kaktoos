package variable

import (
	"fmt"
	"regexp"
)

var placeholderRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Substitute takes a string containing {{placeholders}} and substitutes the values
// using the provided Store. It performs a single-pass substitution, ensuring
// replacements are treated as literals immediately.
// If a variable is referenced but not found in the store, it returns an error
// naming the variable and the step where it was requested.
func Substitute(template string, s *Store, stepName string) (string, error) {
	var err error
	result := placeholderRegex.ReplaceAllStringFunc(template, func(match string) string {
		// match includes the {{ and }}
		// example: {{name}}
		// match[0] is {
		// match[1] is {
		// match[2] is n
		// ...
		// match[len-2] is m
		// match[len-1] is }
		// match[len] is }
		identifier := match[2 : len(match)-2]
		val, ok := s.Get(identifier)
		if !ok {
			err = fmt.Errorf("variable %s not found in step %s", identifier, stepName)
			return match
		}
		return val
	})

	if err != nil {
		return "", err
	}

	return result, nil
}

// SubstituteMap applies substitution to each value in the map.
func SubstituteMap(m map[string]string, s *Store, stepName string) (map[string]string, error) {
	newMap := make(map[string]string)
	for k, v := range m {
		subbed, err := Substitute(v, s, stepName)
		if err != nil {
			return nil, err
		}
		newMap[k] = subbed
	}
	return newMap, nil
}
