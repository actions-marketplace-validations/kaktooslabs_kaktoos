package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/change"
	"github.com/kaktooslabs/kaktoos/internal/contextmodel"
	"github.com/kaktooslabs/kaktoos/internal/impact"
)

func sampleImpact() impact.Impact {
	changed := contextmodel.File("payment-service/fee.go")
	op := contextmodel.Operation("POST", "/payments", "createPayment", "payment-service/openapi.yml")
	return impact.Impact{
		Changed: []contextmodel.Entity{changed},
		Potential: []impact.Affected{{
			Entity: op,
			Path: []contextmodel.Relation{
				{From: changed.ID, To: contextmodel.ServiceID("payment-service"), Kind: contextmodel.RelContains, Why: "matches paths glob payment-service/**"},
				{From: contextmodel.ServiceID("payment-service"), To: op.ID, Kind: contextmodel.RelExposes, Why: "declared in payment-service/openapi.yml"},
			},
		}},
		Owners: []impact.Owner{{Name: "@payments-team", Files: []string{"payment-service/fee.go"}}},
		Plan: impact.VerificationPlan{Workflows: []impact.PlannedWorkflow{
			{Name: "payment-create", File: "payment-service/scenarios/payment-create.yml"},
		}},
	}
}

func TestWriteImpactTextSeparatesChangedFromPotential(t *testing.T) {
	var buf bytes.Buffer
	writeImpactText(&buf, "HEAD", sampleImpact(), nil, false)
	out := buf.String()

	changedIdx := strings.Index(out, "Changed")
	potentialIdx := strings.Index(out, "Potential impact")
	if changedIdx == -1 || potentialIdx == -1 || changedIdx > potentialIdx {
		t.Fatalf("expected Changed section before Potential impact section:\n%s", out)
	}
	if !strings.Contains(out, "payment-service/fee.go") {
		t.Fatalf("changed file missing: %s", out)
	}
	if !strings.Contains(out, "POST /payments") {
		t.Fatalf("potential impact entry missing: %s", out)
	}
	if !strings.Contains(out, "not proof of a defect") {
		t.Fatalf("potential impact must be labeled as potential: %s", out)
	}
	if strings.Contains(out, "broken") {
		t.Fatalf("must never claim something is broken: %s", out)
	}
}

func TestWriteImpactTextWhyShowsRelationChain(t *testing.T) {
	var buf bytes.Buffer
	writeImpactText(&buf, "HEAD", sampleImpact(), nil, true)
	out := buf.String()
	if !strings.Contains(out, "via exposes: declared in payment-service/openapi.yml") {
		t.Fatalf("--why should print the relation chain: %s", out)
	}
}

func TestWriteImpactJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := writeImpactJSON(&buf, "HEAD", sampleImpact(), []string{"PROJ-42"}); err != nil {
		t.Fatal(err)
	}
	var decoded impactJSON
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if decoded.Ref != "HEAD" {
		t.Fatalf("ref: %q", decoded.Ref)
	}
	if len(decoded.Changed) != 1 || len(decoded.Potential) != 1 || len(decoded.Owners) != 1 || len(decoded.Plan.Workflows) != 1 {
		t.Fatalf("decoded shape: %+v", decoded)
	}
	if len(decoded.WorkItems) != 1 || decoded.WorkItems[0] != "PROJ-42" {
		t.Fatalf("work items: %v", decoded.WorkItems)
	}
}

func TestImpactSourceDefaultsToWorkingTree(t *testing.T) {
	resetImpactFlags(t)
	src, err := impactSource()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.(change.WorkingTree); !ok {
		t.Fatalf("expected WorkingTree, got %T", src)
	}
}

func TestImpactSourcePicksCommit(t *testing.T) {
	resetImpactFlags(t)
	impactCommit = "abc123"
	src, err := impactSource()
	if err != nil {
		t.Fatal(err)
	}
	c, ok := src.(change.Commit)
	if !ok || c.SHA != "abc123" {
		t.Fatalf("expected Commit{abc123}, got %#v", src)
	}
}

func TestImpactSourceRejectsMultipleSelectors(t *testing.T) {
	resetImpactFlags(t)
	impactCommit = "abc123"
	impactPR = 7
	if _, err := impactSource(); err == nil {
		t.Fatal("expected an error when more than one selector is given")
	}
}

// TestVerifyPlanRunsExistingWorkflow exercises --verify end to end: it runs
// the plan's scenario through the real engine against a live test server,
// proving verifyPlan reuses engine.RunScenario rather than adding a second
// execution path.
func TestVerifyPlanRunsExistingWorkflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	dir := t.TempDir()
	openapiPath := filepath.Join(dir, "openapi.yml")
	envPath := filepath.Join(dir, "env.yml")
	scenarioPath := filepath.Join(dir, "payment-create.yml")

	os.WriteFile(openapiPath, []byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /payments:
    post:
      operationId: createPayment
      responses: {"201": {description: created}}
`), 0o644)
	os.WriteFile(envPath, []byte("base_url: "+srv.URL+"\n"), 0o644)
	os.WriteFile(scenarioPath, []byte(`name: payment-create
steps:
  - name: create
    operation: createPayment
    request:
      body: '{}'
    assert:
      status: 201
`), 0o644)

	// --openapi/--env are paths as the user gave them (same as `kaktoos run`);
	// only the plan's workflow files are repo-relative.
	impactRepo, impactOpenAPI, impactEnv = dir, openapiPath, envPath
	t.Cleanup(func() { impactRepo, impactOpenAPI, impactEnv = ".", "", "" })

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	plan := impact.VerificationPlan{Workflows: []impact.PlannedWorkflow{
		{Name: "payment-create", File: "payment-create.yml"},
	}}
	if err := verifyPlan(cmd, plan); err != nil {
		t.Fatalf("verifyPlan: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "PASS") {
		t.Fatalf("expected the workflow to pass, got:\n%s", out.String())
	}
}

// An empty plan must not be an error: nothing to verify is a valid outcome.
func TestVerifyPlanEmptyPlanIsNotAFailure(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := verifyPlan(cmd, impact.VerificationPlan{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing to verify") {
		t.Fatalf("out: %s", out.String())
	}
}

func resetImpactFlags(t *testing.T) {
	t.Helper()
	impactCommit, impactBase, impactHead, impactPR = "", "", "", 0
	t.Cleanup(func() { impactCommit, impactBase, impactHead, impactPR = "", "", "", 0 })
}
