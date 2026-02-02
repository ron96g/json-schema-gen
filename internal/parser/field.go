package parser

import (
	"go/ast"
	"reflect"
	"strings"
)

var (
	commonTags = []string{"json", "yaml", "xml", "mapstructure", "validate", "description", "schema"}
)

// parseField extracts FieldInfo from an AST field.
func (p *Parser) parseField(field *ast.Field, nameTag string) []FieldInfo {
	var fields []FieldInfo

	// Get field documentation
	doc := extractDoc(field.Doc, field.Comment)

	// Parse struct tags
	tags := parseTags(field.Tag)

	// Get property name from specified tag
	propertyName, omitEmpty := extractPropertyName(tags, nameTag)

	// Parse the type
	typeInfo := p.parseTypeExpr(field.Type)

	// Handle embedded fields (no names)
	if len(field.Names) == 0 {
		fieldInfo := FieldInfo{
			Name:       typeInfo.Name,
			Type:       typeInfo,
			Tags:       tags,
			Doc:        doc,
			IsEmbedded: true,
			OmitEmpty:  omitEmpty,
		}
		if propertyName != "" {
			fieldInfo.PropertyName = propertyName
		} else {
			fieldInfo.PropertyName = typeInfo.Name
		}
		fields = append(fields, fieldInfo)
		return fields
	}

	// Handle named fields
	for _, name := range field.Names {
		// Skip unexported fields
		if !name.IsExported() {
			continue
		}

		fieldInfo := FieldInfo{
			Name:      name.Name,
			Type:      typeInfo,
			Tags:      tags,
			Doc:       doc,
			OmitEmpty: omitEmpty,
		}

		// Use tag name or fall back to field name
		if propertyName != "" {
			fieldInfo.PropertyName = propertyName
		} else {
			fieldInfo.PropertyName = name.Name
		}

		fields = append(fields, fieldInfo)
	}

	return fields
}

// parseTags parses struct tags into a map.
func parseTags(tagLit *ast.BasicLit) map[string]string {
	tags := make(map[string]string)
	if tagLit == nil {
		return tags
	}

	// Remove backticks
	tagStr := strings.Trim(tagLit.Value, "`")
	if tagStr == "" {
		return tags
	}

	// Use reflect.StructTag to parse
	structTag := reflect.StructTag(tagStr)

	// Extract common tags
	for _, key := range commonTags {
		if val, ok := structTag.Lookup(key); ok {
			tags[key] = val
		}
	}

	return tags
}

// extractPropertyName extracts the property name from a tag.
func extractPropertyName(tags map[string]string, nameTag string) (string, bool) {
	tagValue, ok := tags[nameTag]
	if !ok {
		return "", false
	}

	// Handle json tag format: "name,omitempty"
	parts := strings.Split(tagValue, ",")
	name := parts[0]

	// Check for "-" which means skip
	if name == "-" {
		return "-", false
	}

	// Check for omitempty
	omitEmpty := false
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
			break
		}
	}

	return name, omitEmpty
}

// extractDoc extracts documentation from AST comments.
func extractDoc(doc *ast.CommentGroup, comment *ast.CommentGroup) string {
	var comments []string

	// Prefer doc comments (above the field)
	if doc != nil {
		for _, c := range doc.List {
			text := strings.TrimPrefix(c.Text, "//")
			text = strings.TrimPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
			text = strings.TrimSpace(text)
			if text != "" {
				comments = append(comments, text)
			}
		}
	}

	// Also check line comments (beside the field)
	if len(comments) == 0 && comment != nil {
		for _, c := range comment.List {
			text := strings.TrimPrefix(c.Text, "//")
			text = strings.TrimSpace(text)
			if text != "" {
				comments = append(comments, text)
			}
		}
	}

	return strings.Join(comments, " ")
}
