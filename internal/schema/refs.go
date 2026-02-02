package schema

import (
	"fmt"
	"strings"
)

// RefTracker tracks $ref references to other schemas.
type RefTracker struct {
	refs     map[string]bool // Set of referenced type names
	basePath string          // Base path for relative references
}

// NewRefTracker creates a new RefTracker.
func NewRefTracker() *RefTracker {
	return &RefTracker{
		refs: make(map[string]bool),
	}
}

// AddRef records a reference to another type.
func (rt *RefTracker) AddRef(typeName string) {
	rt.refs[typeName] = true
}

// GetRefs returns all recorded references.
func (rt *RefTracker) GetRefs() []string {
	refs := make([]string, 0, len(rt.refs))
	for ref := range rt.refs {
		refs = append(refs, ref)
	}
	return refs
}

// HasRef checks if a type is referenced.
func (rt *RefTracker) HasRef(typeName string) bool {
	return rt.refs[typeName]
}

// GetRefPath returns the $ref path for a type name.
func (rt *RefTracker) GetRefPath(typeName string) string {
	// Use relative file reference
	return fmt.Sprintf("%s.schema.json", strings.ToLower(typeName))
}

// Clear removes all tracked references.
func (rt *RefTracker) Clear() {
	rt.refs = make(map[string]bool)
}

// DependencyGraph tracks dependencies between types for ordering generation.
type DependencyGraph struct {
	dependencies map[string][]string // type -> types it depends on
}

// NewDependencyGraph creates a new DependencyGraph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		dependencies: make(map[string][]string),
	}
}

// AddDependency records that 'from' depends on 'to'.
func (dg *DependencyGraph) AddDependency(from, to string) {
	dg.dependencies[from] = append(dg.dependencies[from], to)
}

// GetDependencies returns the types that 'typeName' depends on.
func (dg *DependencyGraph) GetDependencies(typeName string) []string {
	return dg.dependencies[typeName]
}

// TopologicalSort returns types in order of dependencies (dependencies first).
// Returns an error if there are circular dependencies.
func (dg *DependencyGraph) TopologicalSort(types []string) ([]string, error) {
	// Build a set of all types
	typeSet := make(map[string]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	// Track visited and in-progress for cycle detection
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)
	var result []string

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if inProgress[name] {
			return fmt.Errorf("circular dependency detected involving type: %s", name)
		}

		inProgress[name] = true

		for _, dep := range dg.dependencies[name] {
			// Only visit dependencies that are in our type set
			if typeSet[dep] {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		inProgress[name] = false
		visited[name] = true
		result = append(result, name)
		return nil
	}

	for _, t := range types {
		if err := visit(t); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// DetectCircular checks for circular dependencies and returns the cycle if found.
func (dg *DependencyGraph) DetectCircular() ([]string, bool) {
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)
	var cycle []string

	var visit func(name string, path []string) bool
	visit = func(name string, path []string) bool {
		if inProgress[name] {
			// Found cycle - find where it starts
			for i, p := range path {
				if p == name {
					cycle = append(path[i:], name)
					return true
				}
			}
			return true
		}
		if visited[name] {
			return false
		}

		inProgress[name] = true
		path = append(path, name)

		for _, dep := range dg.dependencies[name] {
			if visit(dep, path) {
				return true
			}
		}

		inProgress[name] = false
		visited[name] = true
		return false
	}

	for typeName := range dg.dependencies {
		if visit(typeName, nil) {
			return cycle, true
		}
	}

	return nil, false
}
