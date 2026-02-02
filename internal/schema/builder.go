package schema

import (
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/ron96g/json-schema-gen/internal/parser"
)

const (
	// JSONSchemaDraft is the JSON Schema draft version.
	JSONSchemaDraft = "https://json-schema.org/draft/2020-12/schema"
)

// Builder builds JSON Schemas from parsed struct information.
type Builder struct {
	mapper    *ValidatorMapper
	schemaID  string                       // Base URL for $id field
	structMap map[string]parser.StructInfo // Map of struct names for inline lookups
}

// NewBuilder creates a new Builder.
func NewBuilder(schemaID string) *Builder {
	return &Builder{
		mapper:   NewValidatorMapper(),
		schemaID: schemaID,
	}
}

// SetStructMap configures the builder with struct information for per-struct inline support.
// Only structs marked with +schema:inline will have their references inlined.
func (b *Builder) SetStructMap(structMap map[string]parser.StructInfo) {
	b.structMap = structMap
}

// BuildSchema creates a JSON Schema from a StructInfo.
func (b *Builder) BuildSchema(structInfo parser.StructInfo, refTracker *RefTracker) (*jsonschema.Schema, error) {
	// Create inline context for per-struct inline via +schema:inline
	var inlineCtx *InlineContext
	if b.structMap != nil {
		inlineCtx = &InlineContext{
			Enabled:      false,             // No global inline mode
			ParentInline: structInfo.Inline, // per-struct +schema:inline preference
			StructMap:    b.structMap,
			InProgress:   make(map[string]bool),
			Builder:      b,
		}
		// Mark the current struct as in-progress to detect self-references
		inlineCtx.InProgress[structInfo.Name] = true
	}

	schema := &jsonschema.Schema{
		Version: JSONSchemaDraft,
		Title:   structInfo.Name,
		Type:    "object",
	}

	// Set $id if base URL is provided (uses lowercase to match output filename)
	if b.schemaID != "" {
		schema.ID = jsonschema.ID(b.schemaID + "/" + strings.ToLower(structInfo.Name) + ".schema.json")
	}

	// Set description from doc comment
	if structInfo.Doc != "" {
		schema.Description = structInfo.Doc
	}

	// Build properties
	properties := jsonschema.NewProperties()
	var required []string

	for _, field := range structInfo.Fields {
		// Build field schema
		fieldSchema, err := BuildFieldSchema(field, refTracker, inlineCtx)
		if err != nil {
			return nil, err
		}

		// Apply validator constraints
		isRequired := b.mapper.ApplyValidation(fieldSchema, field)
		if isRequired && !field.OmitEmpty {
			required = append(required, field.PropertyName)
		}

		// Add to properties
		properties.Set(field.PropertyName, fieldSchema)
	}

	schema.Properties = properties
	if len(required) > 0 {
		schema.Required = required
	}

	return schema, nil
}

// BuildSchemaWithRefs creates a JSON Schema and returns all referenced types.
// Note: This method is used for dependency tracking, so it always collects refs
// regardless of per-struct inline settings.
func (b *Builder) BuildSchemaWithRefs(structInfo parser.StructInfo) (*jsonschema.Schema, []string, error) {
	refTracker := NewRefTracker()
	// Create a modified structInfo without inline to collect all refs
	nonInlineInfo := structInfo
	nonInlineInfo.Inline = false
	schema, err := b.BuildSchema(nonInlineInfo, refTracker)
	if err != nil {
		return nil, nil, err
	}
	return schema, refTracker.GetRefs(), nil
}

// buildInlineSchema creates an inline schema for a struct (used in inline mode).
func (b *Builder) buildInlineSchema(structInfo parser.StructInfo, inlineCtx *InlineContext) (*jsonschema.Schema, error) {
	schema := &jsonschema.Schema{
		Type: "object",
	}

	// Set description from doc comment
	if structInfo.Doc != "" {
		schema.Description = structInfo.Doc
	}

	// Build properties
	properties := jsonschema.NewProperties()
	var required []string

	for _, field := range structInfo.Fields {
		// Build field schema with inline context
		fieldSchema, err := BuildFieldSchema(field, nil, inlineCtx)
		if err != nil {
			return nil, err
		}

		// Apply validator constraints
		isRequired := b.mapper.ApplyValidation(fieldSchema, field)
		if isRequired && !field.OmitEmpty {
			required = append(required, field.PropertyName)
		}

		// Add to properties
		properties.Set(field.PropertyName, fieldSchema)
	}

	schema.Properties = properties
	if len(required) > 0 {
		schema.Required = required
	}

	return schema, nil
}
