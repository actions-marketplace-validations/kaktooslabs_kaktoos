package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureRepo = "../discovery/testdata/monorepo"

// gitFixture copies the discovery fixture into a temp dir and makes it a real
// git repository with one commit, so change-based tools have something to read.
func gitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("cp", "-R", fixtureRepo+"/.", dir).CombinedOutput(); err != nil {
		t.Fatalf("copy fixture: %v\n%s", err, out)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("add", "-A")
	run("commit", "-q", "-m", "initial commit")
	return dir
}

func TestGetRelatedContextFindsPaymentArea(t *testing.T) {
	res, out, err := GetRelatedContext(context.Background(), nil, GetRelatedContextInput{
		Query: "scheduled payment cancellation", Repo: fixtureRepo,
	})
	if err != nil || res != nil {
		t.Fatalf("unexpected failure: res=%v err=%v", res, err)
	}
	if len(out.Matched) == 0 {
		t.Fatal("expected the payment area to match")
	}
	// payment-create matches the query directly (its name/file mention
	// "payment"), so it's a seed in Matched rather than a reached Related.
	var sawWorkflow bool
	for _, w := range out.Matched {
		if w.Name == "payment-create" {
			sawWorkflow = true
		}
	}
	if !sawWorkflow {
		t.Fatalf("expected payment-create among matched, got %+v", out.Matched)
	}
	for _, e := range out.Explanations {
		if e.Why == "" {
			t.Error("every explanation must carry a why")
		}
	}
}

// The tool must say plainly that nothing matched rather than inventing links.
func TestGetRelatedContextNoMatchInventsNothing(t *testing.T) {
	_, out, err := GetRelatedContext(context.Background(), nil, GetRelatedContextInput{
		Query: "quantum teleportation subsystem", Repo: fixtureRepo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matched) != 0 || len(out.Related) != 0 {
		t.Fatalf("expected no results, got matched=%+v related=%+v", out.Matched, out.Related)
	}
	joined := strings.Join(out.Notes, " ")
	if !strings.Contains(joined, "nothing in this repository matched") {
		t.Fatalf("expected an explicit no-match note, got %v", out.Notes)
	}
}

func TestGetRelatedContextRequiresQuery(t *testing.T) {
	res, _, err := GetRelatedContext(context.Background(), nil, GetRelatedContextInput{Repo: fixtureRepo})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("expected a tool error for a missing query: res=%v err=%v", res, err)
	}
}

func TestGetDependencies(t *testing.T) {
	_, out, err := GetDependencies(context.Background(), nil, GetDependenciesInput{
		EntityID: "service:checkout-service", Repo: fixtureRepo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.DependsOn) != 1 || out.DependsOn[0].Name != "payment-service" {
		t.Fatalf("depends_on: %+v", out.DependsOn)
	}

	_, payment, err := GetDependencies(context.Background(), nil, GetDependenciesInput{
		EntityID: "service:payment-service", Repo: fixtureRepo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(payment.ConsumedBy) != 1 || payment.ConsumedBy[0].Name != "checkout-service" {
		t.Fatalf("consumed_by: %+v", payment.ConsumedBy)
	}
	if len(payment.Exposes) != 2 {
		t.Fatalf("expected two exposed operations, got %+v", payment.Exposes)
	}
}

func TestGetDependenciesUnknownEntity(t *testing.T) {
	res, _, err := GetDependencies(context.Background(), nil, GetDependenciesInput{
		EntityID: "service:nope", Repo: fixtureRepo,
	})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("expected a tool error for an unknown entity: res=%v err=%v", res, err)
	}
}

func TestGetOwners(t *testing.T) {
	_, out, err := GetOwners(context.Background(), nil, GetOwnersInput{
		Paths: []string{"payment-service/fee.go", "nothing/here.txt"}, Repo: fixtureRepo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 2 {
		t.Fatalf("result: %+v", out.Result)
	}
	if len(out.Result[0].Owners) != 1 || out.Result[0].Owners[0] != "@payments-team" {
		t.Fatalf("payment owners: %+v", out.Result[0])
	}
	// The fixture has a catch-all rule, so the second path still resolves —
	// what matters is that ownership is labeled as contextual.
	if !strings.Contains(out.Note, "not a guarantee") {
		t.Fatalf("ownership must be labeled contextual, got %q", out.Note)
	}
}

func TestGetRelatedWorkExtractsKeysLocally(t *testing.T) {
	_, out, err := GetRelatedWork(context.Background(), nil, GetRelatedWorkInput{
		Text: "feature/PROJ-42-cancel-payments",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Available || len(out.Items) != 1 || out.Items[0].Key != "PROJ-42" {
		t.Fatalf("out: %+v", out)
	}
}

// With no documentation provider configured the tool must report unavailable
// rather than implying no documentation exists.
func TestGetRelatedDocumentsReportsUnavailable(t *testing.T) {
	res, out, err := GetRelatedDocuments(context.Background(), nil, GetRelatedDocumentsInput{Terms: []string{"payments"}})
	if err != nil || res != nil {
		t.Fatalf("unavailable provider must not fail the call: res=%v err=%v", res, err)
	}
	if out.Available {
		t.Fatal("expected available=false")
	}
	if out.Unavailable == "" {
		t.Fatal("expected a reason explaining why documents are unavailable")
	}
	if len(out.Documents) != 0 {
		t.Fatalf("documents: %+v", out.Documents)
	}
}

func TestAnalyzeChangeSeparatesChangedFromPotential(t *testing.T) {
	repo := gitFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "payment-service/fee.go"),
		[]byte("package payment\n\nfunc Fee(a int) int { return a / 50 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, out, err := AnalyzeChange(context.Background(), nil, AnalyzeChangeInput{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Changed) != 1 || out.Changed[0].Name != "payment-service/fee.go" {
		t.Fatalf("changed: %+v", out.Changed)
	}
	var sawOperation bool
	for _, p := range out.Potential {
		if p.Entity.Name == "POST /payments" {
			sawOperation = true
		}
		if len(p.Path) == 0 {
			t.Errorf("%s reached with no explanation path", p.Entity.ID)
		}
	}
	if !sawOperation {
		t.Fatalf("expected POST /payments in potential impact, got %+v", out.Potential)
	}
	if !strings.Contains(out.Disclaimer, "not what is broken") {
		t.Fatalf("disclaimer must separate potential from proven: %q", out.Disclaimer)
	}
}

func TestAnalyzeChangeUnrelatedChangeHasNoPotentialImpact(t *testing.T) {
	repo := gitFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "docs-site/index.md"), []byte("# Docs\n\nmore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, err := AnalyzeChange(context.Background(), nil, AnalyzeChangeInput{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Potential) != 0 {
		t.Fatalf("expected no potential impact, got %+v", out.Potential)
	}
	if len(out.Workflows) != 0 {
		t.Fatalf("expected no workflows, got %+v", out.Workflows)
	}
}

func TestAnalyzeChangeRejectsMultipleSelectors(t *testing.T) {
	res, _, err := AnalyzeChange(context.Background(), nil, AnalyzeChangeInput{Commit: "abc", PR: 2})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("expected a tool error: res=%v err=%v", res, err)
	}
}

func TestGetVerificationPlanSelectsExistingWorkflow(t *testing.T) {
	repo := gitFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "payment-service/fee.go"),
		[]byte("package payment\n\nfunc Fee(a int) int { return a }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, err := GetVerificationPlan(context.Background(), nil, GetVerificationPlanInput{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Workflows) != 1 || out.Workflows[0].Name != "payment-create" {
		t.Fatalf("workflows: %+v", out.Workflows)
	}
	if out.Workflows[0].File != "payment-service/scenarios/payment-create.yml" {
		t.Fatalf("workflow file: %q", out.Workflows[0].File)
	}
	if !strings.Contains(out.Guidance, "never generated") && !strings.Contains(out.Guidance, "none are generated") {
		t.Fatalf("guidance must state nothing is generated: %q", out.Guidance)
	}
}

func TestProposeChangeReturnsGroundedContextOnly(t *testing.T) {
	_, out, err := ProposeChange(context.Background(), nil, ProposeChangeInput{
		Description: "cancel a payment after it is created", Repo: fixtureRepo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Existing.Matched) == 0 {
		t.Fatal("expected existing payment context")
	}
	if !strings.Contains(out.Guidance, "no design is implied") {
		t.Fatalf("guidance: %q", out.Guidance)
	}
}

func TestProposeChangeUnknownAreaSaysGreenfield(t *testing.T) {
	_, out, err := ProposeChange(context.Background(), nil, ProposeChangeInput{
		Description: "quantum teleportation subsystem", Repo: fixtureRepo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Considerations) == 0 || !strings.Contains(strings.Join(out.Considerations, " "), "greenfield") {
		t.Fatalf("considerations: %v", out.Considerations)
	}
}

func TestRunVerificationRequiresScenarioPath(t *testing.T) {
	res, _, err := RunVerification(context.Background(), nil, RunVerificationInput{OpenAPIPath: "x.yml"})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("expected a tool error: res=%v err=%v", res, err)
	}
}

func TestRunVerificationMissingScenarioFile(t *testing.T) {
	res, _, err := RunVerification(context.Background(), nil, RunVerificationInput{
		ScenarioPath: "nope.yml", OpenAPIPath: "x.yml", Repo: t.TempDir(),
	})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("expected a tool error: res=%v err=%v", res, err)
	}
}
