// Package parser provides AST parsing functionality for Go source files.
package parser

// TypeKind represents the kind of Go type.
type TypeKind int

const (
	TypeKindPrimitive TypeKind = iota
	TypeKindStruct
	TypeKindSlice
	TypeKindArray
	TypeKindMap
	TypeKindPointer
	TypeKindInterface
	TypeKindTime
	TypeKindUnknown
)

// TypeInfo holds information about a Go type.
type TypeInfo struct {
	Kind        TypeKind
	Name        string    // Type name (e.g., "string", "User", "[]int")
	PackagePath string    // Full package path for named types
	PackageName string    // Short package name (e.g., "time")
	IsPointer   bool      // Whether this is a pointer type
	ElemType    *TypeInfo // Element type for slices, arrays, pointers, maps
	KeyType     *TypeInfo // Key type for maps
	IsExported  bool      // Whether the type name is exported
}

// StructInfo holds parsed information about a Go struct.
type StructInfo struct {
	Name        string
	Package     string // Package name
	PackagePath string // Full package import path
	Fields      []FieldInfo
	Doc         string // Comment above struct
	FilePath    string // Source file path
}

// FieldInfo holds parsed information about a struct field.
type FieldInfo struct {
	Name         string // Go field name
	PropertyName string // Name from tag (json/yaml)
	Type         TypeInfo
	Tags         map[string]string // All struct tags (validate, json, etc.)
	Doc          string            // Comment above or beside field
	IsEmbedded   bool              // Whether this is an embedded field
	OmitEmpty    bool              // Whether json tag has omitempty
}

// IsPrimitive returns true if the type is a Go primitive.
func (t TypeInfo) IsPrimitive() bool {
	return t.Kind == TypeKindPrimitive
}

// IsStruct returns true if the type is a struct.
func (t TypeInfo) IsStruct() bool {
	return t.Kind == TypeKindStruct
}

// IsCollection returns true if the type is a slice, array, or map.
func (t TypeInfo) IsCollection() bool {
	return t.Kind == TypeKindSlice || t.Kind == TypeKindArray || t.Kind == TypeKindMap
}

// IsTime returns true if the type is time.Time.
func (t TypeInfo) IsTime() bool {
	return t.Kind == TypeKindTime
}

// Underlying returns the underlying type for pointers, or the type itself.
func (t TypeInfo) Underlying() TypeInfo {
	if t.Kind == TypeKindPointer && t.ElemType != nil {
		return t.ElemType.Underlying()
	}
	return t
}
