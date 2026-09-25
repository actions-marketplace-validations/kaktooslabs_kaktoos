// Package discovery builds an engineering context model (internal/contextmodel)
// from a repository: an optional kaktoos.yaml manifest (falling back to
// convention when absent), OpenAPI specs, Kaktoos scenarios, and CODEOWNERS.
// Everything here is deterministic and derived from files already in the repo
// — there is no persistent index and nothing is invented.
package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaktooslabs/kaktoos/internal/contextmodel"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

// Note records a non-fatal problem encountered during discovery (an
// unparseable spec, a scenario step referencing an unknown operation) so
// callers can explain why something is missing instead of silently omitting it.
type Note struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

// service is the resolved view of one service regardless of where it came
// from (kaktoos.yaml or convention).
type service struct {
	name      string
	paths     []string
	openapi   string
	scenarios string
	dependsOn []string
	inferred  bool
}

// Discover builds the engineering context model for the repository at root.
func Discover(root string) (*contextmodel.Model, []Note, error) {
	m := contextmodel.New()
	var notes []Note

	services, err := resolveServices(root)
	if err != nil {
		return nil, nil, err
	}

	owners, err := LoadCodeowners(root)
	if err != nil {
		return nil, nil, err
	}

	for _, svc := range services {
		m.Add(contextmodel.Service(svc.name))

		if svc.openapi != "" {
			ops, err := openapi.Load(filepath.Join(root, svc.openapi))
			if err != nil {
				notes = append(notes, Note{Source: svc.openapi, Message: err.Error()})
			} else {
				addOperations(m, svc, ops)
			}
		}

		if svc.scenarios != "" {
			addWorkflows(m, root, svc, &notes)
		}

		for _, dep := range svc.dependsOn {
			m.Relate(contextmodel.ServiceID(svc.name), contextmodel.ServiceID(dep), contextmodel.RelDependsOn,
				"declared depends_on in kaktoos.yaml", svc.inferred)
		}
	}

	addFilesAndOwnership(m, root, services, owners)

	return m, notes, nil
}

// resolveServices returns the declared services from kaktoos.yaml, or a
// convention-derived guess (any directory containing an OpenAPI spec) when no
// manifest exists.
func resolveServices(root string) ([]service, error) {
	if cfgPath := FindConfig(root); cfgPath != "" {
		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			return nil, err
		}
		out := make([]service, 0, len(cfg.Services))
		for _, s := range cfg.Services {
			out = append(out, service{
				name:      s.Name,
				paths:     s.Paths,
				openapi:   s.OpenAPI,
				scenarios: s.Scenarios,
				dependsOn: s.DependsOn,
				inferred:  false,
			})
		}
		return out, nil
	}
	return conventionServices(root)
}

// conventionServices treats any directory containing an openapi.yml/yaml as a
// service named after that directory. Relations derived this way are marked
// inferred.
func conventionServices(root string) ([]service, error) {
	var out []service
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" || d.Name() == "node_modules" {
			return fs.SkipDir
		}
		spec := findOpenAPIFile(path)
		if spec == "" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		name := filepath.Base(path)
		if rel == "." {
			name = filepath.Base(root)
		}
		svc := service{
			name:     name,
			paths:    []string{rel + "/**"},
			openapi:  filepath.ToSlash(mustRel(root, spec)),
			inferred: true,
		}
		if sd := filepath.Join(path, "scenarios"); isDir(sd) {
			svc.scenarios = filepath.ToSlash(mustRel(root, sd))
		}
		out = append(out, svc)
		return fs.SkipDir
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func findOpenAPIFile(dir string) string {
	for _, name := range []string{"openapi.yml", "openapi.yaml"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func addOperations(m *contextmodel.Model, svc service, ops openapi.OperationMap) {
	for path, methods := range ops {
		for method, op := range methods {
			ent := contextmodel.Operation(strings.ToUpper(method), path, op.Name, svc.openapi)
			m.Add(ent)
			m.Relate(contextmodel.ServiceID(svc.name), ent.ID, contextmodel.RelExposes,
				"declared in "+svc.openapi, svc.inferred)
		}
	}
}

func addWorkflows(m *contextmodel.Model, root string, svc service, notes *[]Note) {
	dir := filepath.Join(root, svc.scenarios)
	entries, err := os.ReadDir(dir)
	if err != nil {
		*notes = append(*notes, Note{Source: svc.scenarios, Message: err.Error()})
		return
	}

	var ops openapi.OperationMap
	if svc.openapi != "" {
		ops, _ = openapi.Load(filepath.Join(root, svc.openapi))
	}

	for _, e := range entries {
		if e.IsDir() || !(strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		file := filepath.Join(svc.scenarios, e.Name())
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			*notes = append(*notes, Note{Source: file, Message: err.Error()})
			continue
		}
		scn, err := scenario.Load(data)
		if err != nil {
			*notes = append(*notes, Note{Source: file, Message: err.Error()})
			continue
		}
		ent := contextmodel.Workflow(scn.Name, file)
		m.Add(ent)
		// Both directions are recorded: forward so callers can ask "what does
		// this workflow cover?", reverse so impact analysis walking outward
		// from a changed file can find the workflow that verifies it.
		m.Relate(ent.ID, contextmodel.ServiceID(svc.name), contextmodel.RelVerifies,
			"scenario "+file+" belongs to "+svc.name, svc.inferred)
		m.Relate(contextmodel.ServiceID(svc.name), ent.ID, contextmodel.RelVerifiedBy,
			"verified by scenario "+file, svc.inferred)
		for _, step := range scn.Steps {
			if ops == nil {
				continue
			}
			op, path, method, found := openapi.FindOperation(ops, step.Operation)
			if !found {
				*notes = append(*notes, Note{Source: file, Message: "step " + step.Name + ": operation " + step.Operation + " not found in " + svc.openapi})
				continue
			}
			opID := contextmodel.OperationID(strings.ToUpper(method), path)
			m.Relate(ent.ID, opID, contextmodel.RelVerifies,
				"step "+step.Name+" operation "+op.Name, svc.inferred)
			m.Relate(opID, ent.ID, contextmodel.RelVerifiedBy,
				"step "+step.Name+" operation "+op.Name, svc.inferred)
		}
	}
}

// addFilesAndOwnership walks the repository tree, adding a file entity and a
// contains relation for every regular file that falls under one of the
// resolved services' path globs, plus an owned_by relation when CODEOWNERS
// covers it.
func addFilesAndOwnership(m *contextmodel.Model, root string, services []service, owners *Codeowners) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		svc, inferred := matchService(services, rel)
		if svc != nil {
			fileEnt := contextmodel.File(rel)
			m.Add(fileEnt)
			m.Relate(fileEnt.ID, contextmodel.ServiceID(svc.name), contextmodel.RelContains,
				"matches paths glob "+matchedGlob(*svc, rel), inferred)
		}

		if o := owners.Owners(rel); len(o) > 0 {
			fileID := contextmodel.FileID(rel)
			if _, ok := m.Get(fileID); !ok {
				m.Add(contextmodel.File(rel))
			}
			for _, owner := range o {
				m.Add(contextmodel.Owner(owner))
				m.Relate(fileID, contextmodel.OwnerID(owner), contextmodel.RelOwnedBy,
					"CODEOWNERS covers "+rel, false)
			}
		}
		return nil
	})
}

func matchService(services []service, relPath string) (*service, bool) {
	for i := range services {
		if matchedGlob(services[i], relPath) != "" {
			return &services[i], services[i].inferred
		}
	}
	return nil, false
}

func matchedGlob(svc service, relPath string) string {
	for _, g := range svc.paths {
		if ok, _ := filepath.Match(g, relPath); ok {
			return g
		}
		// Support "dir/**" the way filepath.Match cannot (no ** support):
		// treat it as "everything under dir/".
		if strings.HasSuffix(g, "/**") {
			dir := strings.TrimSuffix(g, "/**")
			if relPath == dir || strings.HasPrefix(relPath, dir+"/") {
				return g
			}
		}
	}
	return ""
}
