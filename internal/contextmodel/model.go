// Package contextmodel represents relationships between engineering entities
// (repositories, files, services, APIs, OpenAPI operations, Kaktoos
// workflows, owners, work items, documents) so that other packages can answer
// "what does this touch?" without re-deriving it themselves.
//
// The model is in-memory and rebuilt on every invocation — there is no
// persistent graph database. Every relation carries a human-readable reason
// (Why) and whether it was declared or inferred, so any answer Kaktoos gives
// can be explained.
package contextmodel

// Kind identifies what an Entity represents.
type Kind string

const (
	KindRepository Kind = "repository"
	KindFile       Kind = "file"
	KindService    Kind = "service"
	KindAPI        Kind = "api"
	KindOperation  Kind = "operation"
	KindWorkflow   Kind = "workflow"
	KindOwner      Kind = "owner"
	KindWorkItem   Kind = "work_item"
	KindDocument   Kind = "document"
)

// RelKind identifies how two entities relate.
type RelKind string

const (
	RelContains   RelKind = "contains"    // repository -> file, repository -> service
	RelExposes    RelKind = "exposes"     // service -> api, api -> operation
	RelVerifies   RelKind = "verifies"    // workflow -> operation, workflow -> service
	RelVerifiedBy RelKind = "verified_by" // operation/service -> workflow (reverse of verifies)
	RelOwnedBy    RelKind = "owned_by"    // file/service -> owner
	RelDependsOn  RelKind = "depends_on"  // service -> service
	RelChanges    RelKind = "changes"     // change -> file, change -> api, change -> workflow
)

// Entity is one node in the engineering context model.
type Entity struct {
	Kind Kind   `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
	// Attrs carries kind-specific detail (path, method, file, spec path, …).
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Relation is a directed edge between two entity IDs. Why is mandatory: it is
// the explanation shown back to a caller asking "why is this affected?".
type Relation struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Kind     RelKind `json:"kind"`
	Why      string  `json:"why"`
	Inferred bool    `json:"inferred"`
}

// Model holds entities and relations discovered from a repository.
type Model struct {
	entities  map[string]Entity
	relations []Relation
	// out indexes relations by From, in insertion order, for fast one-hop walks.
	out map[string][]int
}

// New returns an empty model.
func New() *Model {
	return &Model{
		entities: make(map[string]Entity),
		out:      make(map[string][]int),
	}
}

// Add inserts or replaces an entity. Re-adding the same ID overwrites it
// (last write wins) rather than erroring, so discovery sources can layer.
func (m *Model) Add(e Entity) {
	m.entities[e.ID] = e
}

// Get returns the entity for id, if known.
func (m *Model) Get(id string) (Entity, bool) {
	e, ok := m.entities[id]
	return e, ok
}

// Relate records a directed edge. Both ends need not already exist as
// entities — a relation to an unknown ID is kept but Related/Reachable will
// simply be unable to resolve it to an Entity.
func (m *Model) Relate(from, to string, kind RelKind, why string, inferred bool) {
	m.relations = append(m.relations, Relation{From: from, To: to, Kind: kind, Why: why, Inferred: inferred})
	m.out[from] = append(m.out[from], len(m.relations)-1)
}

// Entities returns every entity in the model. Order is not guaranteed.
func (m *Model) Entities() []Entity {
	out := make([]Entity, 0, len(m.entities))
	for _, e := range m.entities {
		out = append(out, e)
	}
	return out
}

// Relations returns every relation in the model, in insertion order.
func (m *Model) Relations() []Relation {
	return m.relations
}

// RelationsFrom returns the relations whose From is id, optionally filtered
// to the given kinds (all kinds when none given).
func (m *Model) RelationsFrom(id string, kinds ...RelKind) []Relation {
	var out []Relation
	for _, idx := range m.out[id] {
		r := m.relations[idx]
		if len(kinds) == 0 || relKindIn(r.Kind, kinds) {
			out = append(out, r)
		}
	}
	return out
}

// Related returns the entities one hop from id via any of kinds (all kinds
// when none given). Unknown target IDs are skipped.
func (m *Model) Related(id string, kinds ...RelKind) []Entity {
	var out []Entity
	for _, r := range m.RelationsFrom(id, kinds...) {
		if e, ok := m.entities[r.To]; ok {
			out = append(out, e)
		}
	}
	return out
}

// Reachable walks outward from the given seed IDs up to maxDepth hops,
// following any relation kind, and returns every entity reached along with
// the relation path taken to first reach it (BFS — shortest path). Seeds
// themselves are not included. Cycle-safe: each entity is visited once.
func (m *Model) Reachable(seeds []string, maxDepth int) map[string][]Relation {
	visited := map[string]bool{}
	for _, s := range seeds {
		visited[s] = true
	}
	paths := map[string][]Relation{}
	frontier := append([]string{}, seeds...)
	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, id := range frontier {
			for _, r := range m.RelationsFrom(id) {
				if visited[r.To] {
					continue
				}
				visited[r.To] = true
				path := append(append([]Relation{}, paths[id]...), r)
				paths[r.To] = path
				next = append(next, r.To)
			}
		}
		frontier = next
	}
	return paths
}

func relKindIn(k RelKind, kinds []RelKind) bool {
	for _, want := range kinds {
		if k == want {
			return true
		}
	}
	return false
}
