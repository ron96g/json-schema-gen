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
	// Parse all paths to collect structs
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

	// Build struct lookup map
	structMap := make(map[string]parser.StructInfo)
	for _, s := range allStructs {
		structMap[s.Name] = s
	}

	// Build dependency graph and collect all refs
	depGraph := schema.NewDependencyGraph()
	allRefs := make(map[string]bool)

	for _, structInfo := range allStructs {
		_, refs := g.builder.BuildSchemaWithRefs(structInfo)
		for _, ref := range refs {
			depGraph.AddDependency(structInfo.Name, ref)
			allRefs[ref] = true
		}
	}

	// Check for circular dependencies
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

	// Check for missing referenced types
	for ref := range allRefs {
		if _, exists := structMap[ref]; !exists {
			fmt.Printf("Warning: referenced type %q not found in parsed files\n", ref)
		}
	}

	// Generate schemas in dependency order
	for _, typeName := range sortedTypes {
		structInfo, ok := structMap[typeName]
		if !ok {
			continue
		}

		refTracker := schema.NewRefTracker()
		jsonSchema := g.builder.BuildSchema(structInfo, refTracker)

		if err := g.writer.WriteSchema(typeName, jsonSchema); err != nil {
			return fmt.Errorf("write schema for %s: %w", typeName, err)
		}
	}

	return nil
}

// GenerateSingle generates a schema for a single struct.
func (g *Generator) GenerateSingle(structInfo parser.StructInfo) error {
	refTracker := schema.NewRefTracker()
	jsonSchema := g.builder.BuildSchema(structInfo, refTracker)

	return g.writer.WriteSchema(structInfo.Name, jsonSchema)
}
