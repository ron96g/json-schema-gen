// Package generator orchestrates schema generation from Go source files.
package generator

import (
	"fmt"

	"github.com/ron96g/json-schema-gen/internal/parser"
	"github.com/ron96g/json-schema-gen/internal/schema"
)

// Generator orchestrates the parsing and schema generation process.
type Generator struct {
	parser    *parser.Parser
	builder   *schema.Builder
	writer    *Writer
	outputDir string
	recursive bool
}

// Config holds generator configuration.
type Config struct {
	OutputDir string
	NameTag   string // Tag for property names (json, yaml, etc.)
	SchemaID  string // Base URL for $id field
	Recursive bool   // Recursively scan directories
}

// NewGenerator creates a new Generator.
func NewGenerator(cfg Config) *Generator {
	return &Generator{
		parser:    parser.NewParser(cfg.NameTag),
		builder:   schema.NewBuilder(cfg.SchemaID),
		writer:    NewWriter(cfg.OutputDir),
		outputDir: cfg.OutputDir,
		recursive: cfg.Recursive,
	}
}

// GenerateFromPaths generates schemas from the given paths.
func (g *Generator) GenerateFromPaths(paths []string) error {
	// Parse all paths to collect annotated structs
	var allStructs []parser.StructInfo
	for _, path := range paths {
		structs, err := g.parser.ParsePathWithOptions(path, g.recursive)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		allStructs = append(allStructs, structs...)
	}

	if len(allStructs) == 0 {
		return fmt.Errorf("no exported structs found in paths: %v", paths)
	}

	// Build struct lookup map and track annotated structs
	structMap := make(map[string]parser.StructInfo)
	annotatedStructs := make(map[string]bool) // Structs with +schema annotation
	for _, s := range allStructs {
		structMap[s.Name] = s
		annotatedStructs[s.Name] = true
	}

	// Build dependency graph and collect all refs
	depGraph := schema.NewDependencyGraph()
	allRefs := make(map[string]bool)

	for _, structInfo := range allStructs {
		_, refs, err := g.builder.BuildSchemaWithRefs(structInfo)
		if err != nil {
			return fmt.Errorf("analyze refs for %s: %w", structInfo.Name, err)
		}
		for _, ref := range refs {
			depGraph.AddDependency(structInfo.Name, ref)
			allRefs[ref] = true
		}
	}

	// Auto-resolve missing referenced types (structs without +schema annotation)
	resolved := make(map[string]bool)
	for {
		foundNew := false
		for ref := range allRefs {
			// Skip if already in structMap or already tried resolving
			if _, exists := structMap[ref]; exists {
				continue
			}
			if resolved[ref] {
				continue
			}
			resolved[ref] = true

			// Skip external package types (contain a dot)
			if containsDot(ref) {
				continue
			}

			// Search for the struct in all paths
			refStruct := g.findReferencedStruct(ref, paths)
			if refStruct == nil {
				fmt.Printf("Warning: referenced type %q not found in parsed files\n", ref)
				continue
			}

			// Add to structMap and allStructs (but NOT to annotatedStructs)
			structMap[ref] = *refStruct
			allStructs = append(allStructs, *refStruct)

			// Collect refs from the newly resolved struct
			_, newRefs, err := g.builder.BuildSchemaWithRefs(*refStruct)
			if err != nil {
				fmt.Printf("Warning: could not analyze refs for %q: %v\n", ref, err)
				continue
			}
			for _, newRef := range newRefs {
				if !allRefs[newRef] {
					allRefs[newRef] = true
					foundNew = true
				}
				depGraph.AddDependency(ref, newRef)
			}
		}
		if !foundNew {
			break // No more new refs to resolve
		}
	}

	// Configure builder with struct map for per-struct inline support
	g.builder.SetStructMap(structMap)

	// Check for circular dependencies (applies to both inline and ref modes)
	if cycle, hasCycle := depGraph.DetectCircular(); hasCycle {
		return fmt.Errorf("circular dependency detected: %v", cycle)
	}

	// Get all type names
	var typeNames []string
	for _, s := range allStructs {
		typeNames = append(typeNames, s.Name)
	}

	// Topologically sort to generate dependencies first
	sortedTypes, err := depGraph.TopologicalSort(typeNames)
	if err != nil {
		return fmt.Errorf("dependency sort: %w", err)
	}

	// Track which structs are needed as schema files (referenced via $ref by non-inline structs)
	refsNeededAsFiles := make(map[string]bool)
	for _, structInfo := range allStructs {
		// If this struct doesn't use inline mode, its references need schema files
		if !structInfo.Inline {
			for _, ref := range depGraph.GetDependencies(structInfo.Name) {
				refsNeededAsFiles[ref] = true
			}
		}
	}

	// Generate schemas in dependency order
	for _, typeName := range sortedTypes {
		structInfo, ok := structMap[typeName]
		if !ok {
			continue
		}

		// Determine if we should generate a schema file for this struct:
		// 1. Annotated structs (+schema or +schema:inline) always get schema files
		// 2. Auto-resolved structs only get schema files if referenced via $ref
		if !annotatedStructs[typeName] && !refsNeededAsFiles[typeName] {
			continue
		}

		refTracker := schema.NewRefTracker()
		jsonSchema, err := g.builder.BuildSchema(structInfo, refTracker)
		if err != nil {
			return fmt.Errorf("build schema for %s: %w", typeName, err)
		}

		if err := g.writer.WriteSchema(typeName, jsonSchema); err != nil {
			return fmt.Errorf("write schema for %s: %w", typeName, err)
		}
	}

	return nil
}

// findReferencedStruct searches for a struct definition in the given paths.
func (g *Generator) findReferencedStruct(name string, paths []string) *parser.StructInfo {
	for _, searchPath := range paths {
		refStruct, err := g.parser.FindStructByName(searchPath, name, g.recursive)
		if err != nil {
			continue
		}
		if refStruct != nil {
			return refStruct
		}
	}
	return nil
}

// containsDot checks if a string contains a dot (external package reference).
func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}

// GenerateSingle generates a schema for a single struct.
func (g *Generator) GenerateSingle(structInfo parser.StructInfo) error {
	refTracker := schema.NewRefTracker()
	jsonSchema, err := g.builder.BuildSchema(structInfo, refTracker)
	if err != nil {
		return fmt.Errorf("build schema: %w", err)
	}

	return g.writer.WriteSchema(structInfo.Name, jsonSchema)
}
