// Package schema handles JSON Schema generation from parsed Go structs.
package schema

import (
	"github.com/invopop/jsonschema"
	"github.com/ron96g/json-schema-gen/internal/parser"
)

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
func BuildFieldSchema(field parser.FieldInfo, refTracker *RefTracker) *jsonschema.Schema {
	schema := &jsonschema.Schema{}

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

	case parser.TypeKindSlice, parser.TypeKindArray:
		schema.Type = "array"
		if underlying.ElemType != nil {
			elemSchema := buildElemSchema(*underlying.ElemType, refTracker)
			schema.Items = elemSchema
		}

	case parser.TypeKindMap:
		schema.Type = "object"
		if underlying.ElemType != nil {
			valueSchema := buildElemSchema(*underlying.ElemType, refTracker)
			schema.AdditionalProperties = valueSchema
		}

	case parser.TypeKindStruct:
		// Reference to another struct
		if underlying.IsExported && underlying.PackageName == "" {
			// Local struct reference
			refTracker.AddRef(underlying.Name)
			schema.Ref = refTracker.GetRefPath(underlying.Name)
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

	return schema
}

// buildElemSchema creates a schema for collection element types.
func buildElemSchema(typeInfo parser.TypeInfo, refTracker *RefTracker) *jsonschema.Schema {
	underlying := typeInfo.Underlying()

	switch underlying.Kind {
	case parser.TypeKindPrimitive:
		schemaType, format := primitiveToSchema(underlying.Name)
		schema := &jsonschema.Schema{Type: schemaType}
		if format != "" {
			schema.Format = format
		}
		return schema

	case parser.TypeKindTime:
		return &jsonschema.Schema{Type: "string", Format: "date-time"}

	case parser.TypeKindStruct:
		if underlying.IsExported && underlying.PackageName == "" {
			refTracker.AddRef(underlying.Name)
			return &jsonschema.Schema{Ref: refTracker.GetRefPath(underlying.Name)}
		}
		return &jsonschema.Schema{Type: "object"}

	case parser.TypeKindSlice, parser.TypeKindArray:
		schema := &jsonschema.Schema{Type: "array"}
		if underlying.ElemType != nil {
			schema.Items = buildElemSchema(*underlying.ElemType, refTracker)
		}
		return schema

	case parser.TypeKindMap:
		schema := &jsonschema.Schema{Type: "object"}
		if underlying.ElemType != nil {
			schema.AdditionalProperties = buildElemSchema(*underlying.ElemType, refTracker)
		}
		return schema

	default:
		return &jsonschema.Schema{}
	}
}
