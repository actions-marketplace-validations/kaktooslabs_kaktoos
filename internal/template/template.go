package template

import (
	"encoding/base64"
	"fmt"
	"github.com/kaktooslabs/kaktoos/internal/variable"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var functionRE = regexp.MustCompile(`\{\{(\w+)\(([^)]*)\)\}\}`)

func Resolve(s string, store *variable.Store, step string) (string, error) {
	var outErr error
	out := functionRE.ReplaceAllStringFunc(s, func(token string) string {
		m := functionRE.FindStringSubmatch(token)
		args := []string{}
		if m[2] != "" {
			for _, a := range strings.Split(m[2], ",") {
				args = append(args, strings.TrimSpace(a))
			}
		}
		need := map[string]int{"now": 0, "toUpper": 1, "toLower": 1, "base64": 1, "urlEncode": 1, "substring": 3, "addDays": 2}
		n, ok := need[m[1]]
		if !ok {
			outErr = fmt.Errorf("template: step %q: unknown function '%s'", step, m[1])
			return token
		}
		if len(args) != n {
			outErr = fmt.Errorf("template: step %q: '%s' requires %d argument(s), got %d", step, m[1], n, len(args))
			return token
		}
		get := func(k string) string {
			v, ok := store.Get(k)
			if !ok {
				outErr = fmt.Errorf("template: step %q: variable %s not found", step, k)
			}
			return v
		}
		switch m[1] {
		case "now":
			return time.Now().UTC().Format(time.RFC3339)
		case "toUpper":
			return strings.ToUpper(get(args[0]))
		case "toLower":
			return strings.ToLower(get(args[0]))
		case "base64":
			return base64.StdEncoding.EncodeToString([]byte(get(args[0])))
		case "urlEncode":
			return url.PathEscape(get(args[0]))
		case "substring":
			v := get(args[0])
			a, e := strconv.Atoi(args[1])
			b, e2 := strconv.Atoi(args[2])
			if e != nil || e2 != nil {
				outErr = fmt.Errorf("template: step %q: 'substring' argument must be an integer", step)
				return token
			}
			if a < 0 {
				a = 0
			}
			if b < 0 || b > len(v) {
				b = len(v)
			}
			if a > len(v) {
				a = len(v)
			}
			if b < a {
				b = a
			}
			return v[a:b]
		case "addDays":
			v := get(args[0])
			n, e := strconv.Atoi(args[1])
			if e != nil {
				outErr = fmt.Errorf("template: step %q: 'addDays' argument 1 must be an integer", step)
				return token
			}
			t, e := time.Parse(time.RFC3339, v)
			if e != nil {
				outErr = fmt.Errorf("template: step %q: 'addDays': cannot parse '%s' as RFC 3339", step, v)
				return token
			}
			return t.AddDate(0, 0, n).UTC().Format(time.RFC3339)
		}
		return token
	})
	if outErr != nil {
		return "", outErr
	}
	return out, nil
}
func ResolveMap(m map[string]string, s *variable.Store, step string) (map[string]string, error) {
	out := make(map[string]string, len(m))
	for k, v := range m {
		r, e := Resolve(v, s, step)
		if e != nil {
			return nil, e
		}
		out[k] = r
	}
	return out, nil
}
