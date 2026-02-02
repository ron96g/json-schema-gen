// Package schema handles JSON Schema generation from parsed Go structs.
package schema

import (
	"fmt"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/ron96g/json-schema-gen/internal/parser"
)

// InlineContext holds state for inline schema generation.
type InlineContext struct {
	Enabled      bool                         // Deprecated: kept for compatibility, always false
	ParentInline bool                         // Whether the parent struct has +schema:inline
	StructMap    map[string]parser.StructInfo // Map of struct names to their info
	InProgress   map[string]bool              // Tracks types being built (circular ref detection)
	Builder      *Builder                     // Reference to builder for recursive calls
}

// GoTypeToJSONSchema converts a Go TypeInfo to JSON Schema type and format.
func GoTypeToJSONSchema(typeInfo parser.TypeInfo) (schemaType string, format string) {
	// Handle pointers - get underlying type
	if typeInfo.Kind == parser.TypeKindPointer && typeInfo.ElemType != nil {
		return GoTypeToJSONSchema(*typeInfo.ElemType)
	}

	switch typeInfo.Kind {
	case parser.TypeKindPrimitive:
		return primitiveToSchema(typeInfo.Name)

	case parser.TypeKindTime:
		return "string", "date-time"

	case parser.TypeKindDuration:
		return "string", "duration"

	case parser.TypeKindAlias:
		// Resolve alias to its underlying type
		return primitiveToSchema(typeInfo.UnderlyingName)

	case parser.TypeKindSlice, parser.TypeKindArray:
		return "array", ""

	case parser.TypeKindMap:
		return "object", ""

	case parser.TypeKindInterface:
		return "", "" // Any type

	case parser.TypeKindStruct:
		return "object", ""

	default:
		return "", ""
	}
}

// primitiveToSchema maps Go primitive types to JSON Schema types.
func primitiveToSchema(name string) (string, string) {
	switch name {
	case "string":
		return "string", ""

	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"byte", "rune":
		return "integer", ""

	case "float32", "float64":
		return "number", ""

	case "bool":
		return "boolean", ""

	default:
		return "string", ""
	}
}

// BuildFieldSchema creates a JSON Schema for a field's type.
// If inlineCtx is provided and enabled, struct references are inlined instead of using $ref.
func BuildFieldSchema(field parser.FieldInfo, refTracker *RefTracker, inlineCtx *InlineContext) (*jsonschema.Schema, error) {
	schema := &jsonschema.Schema{}

	// Check for schema tag override (e.g., schema:"type=string")
	if schemaTag, ok := field.Tags["schema"]; ok {
		if overrideType := parseSchemaTypeOverride(schemaTag); overrideType != "" {
			schema.Type = overrideType
			// Add description from doc comment
			if field.Doc != "" {
				schema.Description = field.Doc
			}
			return schema, nil
		}
	}

	// Handle based on type kind
	underlying := field.Type.Underlying()

	switch underlying.Kind {
	case parser.TypeKindPrimitive:
		schemaType, format := primitiveToSchema(underlying.Name)
		schema.Type = schemaType
		if format != "" {
			schema.Format = format
		}

	case parser.TypeKindTime:
		schema.Type = "string"
		schema.Format = "date-time"

	case parser.TypeKindDuration:
		schema.Type = "string"
		schema.Format = "duration"

	case parser.TypeKindAlias:
		// Resolve alias to underlying primitive type
		schemaType, format := primitiveToSchema(underlying.UnderlyingName)
		schema.Type = schemaType
		if format != "" {
			schema.Format = format
		}

	case parser.TypeKindSlice, parser.TypeKindArray:
		schema.Type = "array"
		if underlying.ElemType != nil {
			elemSchema, err := buildElemSchema(*underlying.ElemType, refTracker, inlineCtx)
			if err != nil {
				return nil, err
			}
			schema.Items = elemSchema
		}

	case parser.TypeKindMap:
		schema.Type = "object"
		if underlying.ElemType != nil {
			valueSchema, err := buildElemSchema(*underlying.ElemType, refTracker, inlineCtx)
			if err != nil {
				return nil, err
			}
			schema.AdditionalProperties = valueSchema
		}

	case parser.TypeKindStruct:
		// Reference to another struct
		if underlying.IsExported && underlying.PackageName == "" {
			// Determine if we should inline this specific struct reference
			shouldInline := shouldInlineStruct(inlineCtx)

			if shouldInline {
				inlinedSchema, err := inlineStructSchema(underlying.Name, inlineCtx)
				if err != nil {
					return nil, err
				}
				if inlinedSchema != nil {
					// Copy relevant fields from inlined schema
					schema.Type = inlinedSchema.Type
					schema.Properties = inlinedSchema.Properties
					schema.Required = inlinedSchema.Required
					schema.Description = inlinedSchema.Description
				} else {
					// Referenced type not found, treat as object
					schema.Type = "object"
				}
			} else {
				// Use $ref
				if refTracker != nil {
					refTracker.AddRef(underlying.Name)
					schema.Ref = refTracker.GetRefPath(underlying.Name)
				} else {
					schema.Type = "object"
				}
			}
		} else if underlying.PackageName != "" {
			// External package struct - treat as object
			schema.Type = "object"
		} else {
			schema.Type = "object"
		}

	case parser.TypeKindInterface:
		// Any type - no constraints

	default:
		schema.Type = "string"
	}

	// Add description from doc comment
	if field.Doc != "" {
		schema.Description = field.Doc
	}

	return schema, nil
}

// shouldInlineStruct determines whether a referenced struct should be inlined.
// Returns true if the parent struct has +schema:inline marker.
func shouldInlineStruct(inlineCtx *InlineContext) bool {
	if inlineCtx == nil {
		return false
	}

	// Only inline if parent struct has +schema:inline
	return inlineCtx.ParentInline
}

// inlineStructSchema creates an inline schema for a referenced struct.
func inlineStructSchema(name string, inlineCtx *InlineContext) (*jsonschema.Schema, error) {
	structInfo, ok := inlineCtx.StructMap[name]
	if !ok {
		// Referenced type not found
		return nil, nil
	}

	// Check for circular reference
	if inlineCtx.InProgress[name] {
		return nil, fmt.Errorf("circular reference detected: %s", name)
	}

	// Mark as in-progress
	inlineCtx.InProgress[name] = true

	// Recursively build inline schema
	inlinedSchema, err := inlineCtx.Builder.buildInlineSchema(structInfo, inlineCtx)
	if err != nil {
		return nil, err
	}

	// Clear in-progress (allow same type to be used in different branches)
	delete(inlineCtx.InProgress, name)

	return inlinedSchema, nil
}

// buildElemSchema creates a schema for collection element types.
func buildElemSchema(typeInfo parser.TypeInfo, refTracker *RefTracker, inlineCtx *InlineContext) (*jsonschema.Schema, error) {
	underlying := typeInfo.Underlying()

	switch underlying.Kind {
	case parser.TypeKindPrimitive:
		schemaType, format := primitiveToSchema(underlying.Name)
		schema := &jsonschema.Schema{Type: schemaType}
		if format != "" {
			schema.Format = format
		}
		return schema, nil

	case parser.TypeKindTime:
		return &jsonschema.Schema{Type: "string", Format: "date-time"}, nil

	case parser.TypeKindDuration:
		return &jsonschema.Schema{Type: "string", Format: "duration"}, nil

	case parser.TypeKindAlias:
		schemaType, format := primitiveToSchema(underlying.UnderlyingName)
		schema := &jsonschema.Schema{Type: schemaType}
		if format != "" {
			schema.Format = format
		}
		return schema, nil

	case parser.TypeKindStruct:
		if underlying.IsExported && underlying.PackageName == "" {
			// Determine if we should inline this specific struct reference
			shouldInline := shouldInlineStruct(inlineCtx)

			if shouldInline {
				inlinedSchema, err := inlineStructSchema(underlying.Name, inlineCtx)
				if err != nil {
					return nil, err
				}
				if inlinedSchema != nil {
					return inlinedSchema, nil
				}
				// Referenced type not found, treat as object
				return &jsonschema.Schema{Type: "object"}, nil
			}
			// Use $ref
			if refTracker != nil {
				refTracker.AddRef(underlying.Name)
				return &jsonschema.Schema{Ref: refTracker.GetRefPath(underlying.Name)}, nil
			}
			return &jsonschema.Schema{Type: "object"}, nil
		}
		return &jsonschema.Schema{Type: "object"}, nil

	case parser.TypeKindSlice, parser.TypeKindArray:
		schema := &jsonschema.Schema{Type: "array"}
		if underlying.ElemType != nil {
			items, err := buildElemSchema(*underlying.ElemType, refTracker, inlineCtx)
			if err != nil {
				return nil, err
			}
			schema.Items = items
		}
		return schema, nil

	case parser.TypeKindMap:
		schema := &jsonschema.Schema{Type: "object"}
		if underlying.ElemType != nil {
			additionalProps, err := buildElemSchema(*underlying.ElemType, refTracker, inlineCtx)
			if err != nil {
				return nil, err
			}
			schema.AdditionalProperties = additionalProps
		}
		return schema, nil

	default:
		return &jsonschema.Schema{}, nil
	}
}

// parseSchemaTypeOverride extracts the type override from a schema tag.
// Supports format: schema:"type=string" or schema:"type=integer"
func parseSchemaTypeOverride(schemaTag string) string {
	for _, part := range strings.Split(schemaTag, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "type=") {
			return strings.TrimPrefix(part, "type=")
		}
	}
	return ""
}
