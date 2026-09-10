package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/idempotency"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/ratelimit"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/variable"
)

// maxBodySize is the frozen maximum webhook request body size (10 MB).
const maxBodySize = 10 << 20

// shutdownTimeout is the frozen graceful shutdown deadline.
const shutdownTimeout = 5 * time.Second

// Server serves webhook routes, executing workflows via the existing engine.
type Server struct {
	config      *Config
	port        int
	host        string
	idempotency idempotency.Store  // shared for the server's lifetime
	rateLimiter *ratelimit.Limiter // shared for the server's lifetime
	client      *http.Client
}

// WebhookResponse is the JSON body returned for a webhook request.
type WebhookResponse struct {
	Status             string            `json:"status"`
	Workflow           string            `json:"workflow,omitempty"`
	ExecutionID        string            `json:"execution_id,omitempty"`
	Duration           string            `json:"duration,omitempty"`
	StartTime          string            `json:"start_time,omitempty"`
	EndTime            string            `json:"end_time,omitempty"`
	Steps              *StepSummary      `json:"steps,omitempty"`
	ExtractedVariables map[string]string `json:"extracted_variables,omitempty"`
	FailedStep         string            `json:"failed_step,omitempty"`
	Error              string            `json:"error,omitempty"`
}

// StepSummary counts step outcomes for a single execution.
type StepSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
	TimedOut int `json:"timed_out"`
}

// NewServer creates a webhook server with ONE idempotency store and ONE rate
// limiter shared by every request for the server's lifetime.
func NewServer(cfg *Config, port int, host string) *Server {
	return &Server{
		config:      cfg,
		port:        port,
		host:        host,
		idempotency: idempotency.NewMemoryStore(),
		rateLimiter: ratelimit.NewLimiter(aggregateRateLimits(cfg)),
		client:      httpclient.NewClient().HTTPClient(),
	}
}

// aggregateRateLimits merges the rate limits declared by every route environment.
// Unreadable environments are skipped here; the per-request handler reports the
// load failure as an infrastructure error.
func aggregateRateLimits(cfg *Config) map[string]config.RateLimitEntry {
	limits := map[string]config.RateLimitEntry{}
	if cfg == nil {
		return limits
	}
	for _, r := range cfg.Routes {
		env, err := config.Load(r.Workflow.EnvironmentPath)
		if err != nil {
			continue
		}
		for op, entry := range env.RateLimits {
			limits[op] = entry
		}
	}
	return limits
}

// Handler returns the HTTP handler for the configured routes.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

// Start listens on host:port and blocks until SIGINT/SIGTERM, then shuts down
// gracefully within shutdownTimeout. Returns nil on clean shutdown.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	srv := &http.Server{Addr: addr, Handler: s.Handler()}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("Listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// 1-2. Route match, then method check.
	var route *Route
	var pathParams map[string]string
	pathMatched := false
	for i := range s.config.Routes {
		params, ok := matchPath(s.config.Routes[i].Path, r.URL.Path)
		if !ok {
			continue
		}
		pathMatched = true
		if s.config.Routes[i].Method == strings.ToUpper(r.Method) {
			route = &s.config.Routes[i]
			pathParams = params
			break
		}
	}
	if route == nil {
		if pathMatched {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		} else {
			writeError(w, http.StatusNotFound, "no webhook route matches "+r.URL.Path)
		}
		return
	}

	// 3. Read body, bounded at 10MB.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read request body")
		return
	}
	if len(body) > maxBodySize {
		writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 10MB limit")
		return
	}

	// 4. Authenticate before any workflow work.
	if !NewAuthenticator(route.Auth).Verify(r, body) {
		writeError(w, http.StatusUnauthorized, "authentication failed")
		return
	}

	// 5. Parse JSON body.
	parsed := map[string]interface{}{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			writeError(w, http.StatusBadRequest, "request body is not a valid JSON object")
			return
		}
	}

	// 6-7. Extract webhook variables.
	webhookVars, err := ExtractVariables(route.VariableMapping, pathParams, parsed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 8. Load workflow files. Failures here are infrastructure errors.
	env, err := config.Load(route.Workflow.EnvironmentPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("loading environment: %v", err))
		return
	}
	ops, err := openapi.Load(route.Workflow.OpenAPIPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("loading OpenAPI spec: %v", err))
		return
	}
	scenarioData, err := os.ReadFile(route.Workflow.ScenarioPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("reading scenario: %v", err))
		return
	}
	scn, err := scenario.Load(scenarioData)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("loading scenario: %v", err))
		return
	}

	// 9. Webhook vars first (lower precedence), env vars second (override).
	store := variable.NewStore()
	store.Seed(webhookVars)
	store.Seed(env.Variables)

	// 10-11. Execute through the existing engine with the shared store/limiter.
	ctx := engine.NewContext(env, ops)
	ctx.VariableStore = store
	ctx.RateLimiter = s.rateLimiter
	ctx.Idempotency = s.idempotency
	ctx.TraceEnabled = false

	result := engine.RunScenario(ctx, *scn, s.client)

	// 12. Workflow results always return HTTP 200.
	writeJSON(w, http.StatusOK, buildResponse(result, store))
}

func buildResponse(result engine.ExecutionResult, store *variable.Store) WebhookResponse {
	summary := StepSummary{Total: len(result.Steps)}
	failedStep := ""
	for _, st := range result.Steps {
		switch st.Status {
		case engine.StepPassed:
			summary.Passed++
		case engine.StepFailed:
			summary.Failed++
			if failedStep == "" {
				failedStep = st.Name
			}
		case engine.StepTimedOut:
			summary.TimedOut++
			if failedStep == "" {
				failedStep = st.Name
			}
		case engine.StepSkipped:
			summary.Skipped++
		}
	}

	return WebhookResponse{
		Status:             string(result.Status),
		Workflow:           result.ScenarioName,
		ExecutionID:        result.ExecutionID,
		Duration:           result.Duration.String(),
		StartTime:          result.StartedAt.UTC().Format(time.RFC3339),
		EndTime:            result.FinishedAt.UTC().Format(time.RFC3339),
		Steps:              &summary,
		ExtractedVariables: store.GetAll(),
		FailedStep:         failedStep,
		Error:              result.Error,
	}
}

// matchPath matches a route path against a request path, capturing {name}
// segments into params.
func matchPath(routePath, requestPath string) (map[string]string, bool) {
	routeSegs := strings.Split(strings.Trim(routePath, "/"), "/")
	reqSegs := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(routeSegs) != len(reqSegs) {
		return nil, false
	}

	params := map[string]string{}
	for i, seg := range routeSegs {
		if len(seg) > 2 && strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params[seg[1:len(seg)-1]] = reqSegs[i]
			continue
		}
		if seg != reqSegs[i] {
			return nil, false
		}
	}
	return params, true
}

func writeJSON(w http.ResponseWriter, code int, body WebhookResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, WebhookResponse{Status: "error", Error: msg})
}
