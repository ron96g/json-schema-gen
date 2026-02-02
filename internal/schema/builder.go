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
	mapper   *ValidatorMapper
	schemaID string // Base URL for $id field
}

// NewBuilder creates a new Builder.
func NewBuilder(schemaID string) *Builder {
	return &Builder{
		mapper:   NewValidatorMapper(),
		schemaID: schemaID,
	}
}

// BuildSchema creates a JSON Schema from a StructInfo.
func (b *Builder) BuildSchema(structInfo parser.StructInfo, refTracker *RefTracker) *jsonschema.Schema {
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
		fieldSchema := BuildFieldSchema(field, refTracker)

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

	return schema
}

// BuildSchemaWithRefs creates a JSON Schema and returns all referenced types.
func (b *Builder) BuildSchemaWithRefs(structInfo parser.StructInfo) (*jsonschema.Schema, []string) {
	refTracker := NewRefTracker()
	schema := b.BuildSchema(structInfo, refTracker)
	return schema, refTracker.GetRefs()
}
