// Package impact turns a change.ChangeSet plus a contextmodel.Model into what
// actually changed and what could be affected. It is a pure function over
// data — no I/O — so that "changed" and "potential impact" stay strictly and
// testably separate: Kaktoos never claims something is broken, only that a
// path connects it to a change.
package impact

import "github.com/kaktooslabs/kaktoos/internal/contextmodel"

// defaultMaxDepth bounds how far potential impact is allowed to spread from
// a changed file. Kept small and fixed: this is heuristic reachability, not
// a claim about actual runtime behavior.
const defaultMaxDepth = 4

// Affected is one entity potentially impacted by a change, with the relation
// chain that explains why (e.g. file -> service -> operation -> workflow).
type Affected struct {
	Entity contextmodel.Entity     `json:"entity"`
	Path   []contextmodel.Relation `json:"path"`
}

// Owner is a CODEOWNERS owner of one or more changed files.
type Owner struct {
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

// VerificationPlan lists existing Kaktoos workflows relevant to a change.
// Nothing here is generated; every entry names a workflow that already exists.
type VerificationPlan struct {
	Workflows []PlannedWorkflow `json:"workflows"`
}

// PlannedWorkflow is one existing scenario selected for verification.
type PlannedWorkflow struct {
	Name   string                  `json:"name"`
	File   string                  `json:"file"`
	Reason []contextmodel.Relation `json:"reason"`
}

// Impact is the result of analyzing a change against an engineering context
// model. Changed is fact; Potential is always presented as potential.
type Impact struct {
	Changed   []contextmodel.Entity `json:"changed"`
	Potential []Affected            `json:"potential"`
	Owners    []Owner               `json:"owners"`
	Plan      VerificationPlan      `json:"verification"`
	Notes     []string              `json:"notes,omitempty"`
}

// ChangedFile is the minimal shape Analyze needs from a change, decoupling
// this package from internal/change's exact type.
type ChangedFile struct {
	Path   string
	Status string
}

// Analyze walks from every changed file through the model and reports what
// it reaches. A file the model has no entity for still counts as Changed —
// it simply reaches nothing.
func Analyze(model *contextmodel.Model, files []ChangedFile) Impact {
	return AnalyzeAtDepth(model, files, defaultMaxDepth)
}

// AnalyzeAtDepth is Analyze with an explicit reachability bound (mainly for
// tests).
func AnalyzeAtDepth(model *contextmodel.Model, files []ChangedFile, maxDepth int) Impact {
	imp := Impact{}
	seeds := make([]string, 0, len(files))

	for _, f := range files {
		id := contextmodel.FileID(f.Path)
		if e, ok := model.Get(id); ok {
			imp.Changed = append(imp.Changed, e)
		} else {
			imp.Changed = append(imp.Changed, contextmodel.Entity{
				Kind: contextmodel.KindFile, ID: id, Name: f.Path,
				Attrs: map[string]string{"path": f.Path, "status": f.Status},
			})
			imp.Notes = append(imp.Notes, "no engineering context found for "+f.Path)
		}
		seeds = append(seeds, id)
	}

	paths := model.Reachable(seeds, maxDepth)
	owners := map[string]*Owner{}
	workflows := map[string]PlannedWorkflow{}

	for id, path := range paths {
		e, ok := model.Get(id)
		if !ok {
			continue
		}
		switch e.Kind {
		case contextmodel.KindOwner:
			// Ownership is "who to loop in", not something impacted by the
			// change — keep it out of Potential, report it separately.
			o := owners[id]
			if o == nil {
				o = &Owner{Name: e.Name}
				owners[id] = o
			}
			o.Files = append(o.Files, ownedFile(path))
			continue
		case contextmodel.KindWorkflow:
			workflows[id] = PlannedWorkflow{Name: e.Name, File: e.Attrs["file"], Reason: path}
		}
		imp.Potential = append(imp.Potential, Affected{Entity: e, Path: path})
	}

	for _, o := range owners {
		imp.Owners = append(imp.Owners, *o)
	}
	for _, w := range workflows {
		imp.Plan.Workflows = append(imp.Plan.Workflows, w)
	}
	return imp
}

// ownedFile returns the changed file at the start of an owner's relation
// path, for reporting which file triggered which owner.
func ownedFile(path []contextmodel.Relation) string {
	if len(path) == 0 {
		return ""
	}
	from := path[0].From
	const prefix = "file:"
	if len(from) > len(prefix) && from[:len(prefix)] == prefix {
		return from[len(prefix):]
	}
	return from
}
