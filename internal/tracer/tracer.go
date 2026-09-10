package tracer

import (
	"encoding/json"
	"fmt"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"io"
	"strings"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

type Tracer struct {
	enabled   bool
	format    Format
	sensitive bool
}

func New(enabled bool, format Format, sensitive bool) *Tracer {
	return &Tracer{enabled, format, sensitive}
}
func (t *Tracer) Write(r engine.ExecutionResult, w io.Writer) error {
	if !t.enabled {
		return nil
	}
	if t.format == FormatJSON {
		return json.NewEncoder(w).Encode(t.sanitize(r))
	}
	_, e := fmt.Fprintf(w, "execution_id: %s\nworkflow: %s\nstatus: %s\n", r.ExecutionID, r.ScenarioName, r.Status)
	return e
}
func (t *Tracer) sanitize(r engine.ExecutionResult) engine.ExecutionResult {
	out := r
	out.Steps = append([]engine.StepResult(nil), r.Steps...)
	for i := range out.Steps {
		out.Steps[i].Attempts = append([]engine.AttemptResult(nil), r.Steps[i].Attempts...)
		for j := range out.Steps[i].Attempts {
			a := &out.Steps[i].Attempts[j]
			a.RequestHeaders = maskHeaders(a.RequestHeaders, t.sensitive)
			a.ResponseHeaders = maskHeaders(a.ResponseHeaders, t.sensitive)
			if len(a.RequestBody) > 2048 {
				a.RequestBody = a.RequestBody[:2048]
			}
			if len(a.ResponseBody) > 2048 {
				a.ResponseBody = a.ResponseBody[:2048]
			}
		}
	}
	return out
}
func maskHeaders(h map[string]string, s bool) map[string]string {
	if s {
		return h
	}
	o := map[string]string{}
	for k, v := range h {
		switch strings.ToLower(k) {
		case "authorization", "x-api-key", "x-auth-token", "cookie", "set-cookie":
			o[k] = "***"
		default:
			o[k] = v
		}
	}
	return o
}
