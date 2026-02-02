package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SchemaMarker is the annotation marker for structs to include in schema generation.
const SchemaMarker = "+schema"

// Parser handles AST parsing of Go source files.
type Parser struct {
	fset         *token.FileSet
	nameTag      string               // Tag to use for property names (json, yaml, etc.)
	typeRegistry map[string]TypeDecl  // Registry of type declarations in current package
	parsedFiles  map[string]*ast.File // Cache of parsed AST files
}

// NewParser creates a new Parser instance.
func NewParser(nameTag string) *Parser {
	if nameTag == "" {
		nameTag = "json"
	}
	return &Parser{
		fset:         token.NewFileSet(),
		nameTag:      nameTag,
		typeRegistry: make(map[string]TypeDecl),
		parsedFiles:  make(map[string]*ast.File),
	}
}

// ParsePath parses Go files from a path (file or directory).
func (p *Parser) ParsePath(path string) ([]StructInfo, error) {
	return p.ParsePathWithOptions(path, false)
}

// ParsePathWithOptions parses Go files with optional recursive scanning.
// Only structs with the // +schema annotation are included.
func (p *Parser) ParsePathWithOptions(path string, recursive bool) ([]StructInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat path %s: %w", path, err)
	}

	if recursive && info.IsDir() {
		return p.parseRecursive(path)
	}

	if info.IsDir() {
		return p.parseDirectory(path)
	}
	return p.parseFile(path)
}

// parseRecursive recursively walks directories and parses all Go packages.
func (p *Parser) parseRecursive(root string) ([]StructInfo, error) {
	var allStructs []StructInfo

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}

		structs, err := p.parseDirectory(path)
		if err != nil {
			// Log warning but continue with other directories
			fmt.Printf("Warning: failed to parse %s: %v\n", path, err)
			return nil
		}
		allStructs = append(allStructs, structs...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory %s: %w", root, err)
	}

	return allStructs, nil
}

// shouldSkipDir returns true for directories that should be skipped during recursive scanning.
func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		"vendor":       true,
		"node_modules": true,
		"testdata":     true,
		".git":         true,
		".svn":         true,
		".hg":          true,
	}
	return skipDirs[name]
}

// parseDirectory parses all Go files in a directory.
func (p *Parser) parseDirectory(dir string) ([]StructInfo, error) {
	var allStructs []StructInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip test files
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		structs, err := p.parseFile(filePath)
		if err != nil {
			return nil, err
		}
		allStructs = append(allStructs, structs...)
	}

	return allStructs, nil
}

// parseFile parses a single Go file.
func (p *Parser) parseFile(filePath string) ([]StructInfo, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	file, err := parser.ParseFile(p.fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %w", filePath, err)
	}

	// Pass 1: Extract type declarations to build registry
	p.extractTypeDecls(file)

	// Pass 2: Extract structs using the registry
	return p.extractStructs(file, filePath)
}

// extractTypeDecls extracts type declarations from an AST file to build the type registry.
// This is the first pass of parsing that identifies type aliases like `type MyEnum string`.
func (p *Parser) extractTypeDecls(file *ast.File) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Only process exported types
			if !typeSpec.Name.IsExported() {
				continue
			}

			// Check if this is a simple type alias (not a struct)
			ident, ok := typeSpec.Type.(*ast.Ident)
			if !ok {
				continue // Skip structs, interfaces, etc.
			}

			// Determine the underlying type kind
			underlyingKind, underlyingName := p.classifyPrimitive(ident.Name)
			if underlyingKind == TypeKindUnknown {
				continue // Skip aliases to non-primitives
			}

			p.typeRegistry[typeSpec.Name.Name] = TypeDecl{
				Name:           typeSpec.Name.Name,
				UnderlyingKind: underlyingKind,
				UnderlyingName: underlyingName,
			}
		}
	}
}

// classifyPrimitive determines the TypeKind and normalized name for a primitive type.
func (p *Parser) classifyPrimitive(name string) (TypeKind, string) {
	switch name {
	case "string":
		return TypeKindPrimitive, "string"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"byte", "rune":
		return TypeKindPrimitive, name
	case "float32", "float64":
		return TypeKindPrimitive, name
	case "bool":
		return TypeKindPrimitive, "bool"
	default:
		return TypeKindUnknown, ""
	}
}

// extractStructs extracts all exported structs from an AST file.
func (p *Parser) extractStructs(file *ast.File, filePath string) ([]StructInfo, error) {
	var structs []StructInfo
	packageName := file.Name.Name

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Only process exported types
			if !typeSpec.Name.IsExported() {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Require +schema annotation
			if !hasSchemaMarker(genDecl.Doc, typeSpec.Doc) {
				continue
			}

			// Extract inline preference from marker
			_, inline := parseSchemaMarker(typeSpec.Doc)
			if !inline {
				_, inline = parseSchemaMarker(genDecl.Doc)
			}

			structInfo := p.parseStruct(typeSpec, structType, packageName, filePath, genDecl.Doc)
			structInfo.Inline = inline
			structs = append(structs, structInfo)
		}
	}

	return structs, nil
}

// hasSchemaMarker checks if the doc comments contain the +schema marker.
func hasSchemaMarker(groupDoc, typeDoc *ast.CommentGroup) bool {
	hasMarker, _ := parseSchemaMarker(typeDoc)
	if hasMarker {
		return true
	}
	hasMarker, _ = parseSchemaMarker(groupDoc)
	return hasMarker
}

// parseSchemaMarker checks for +schema marker and extracts options.
// Returns: hasMarker bool, inline bool
func parseSchemaMarker(cg *ast.CommentGroup) (bool, bool) {
	if cg == nil {
		return false, false
	}
	for _, c := range cg.List {
		text := c.Text
		// Handle both // and /* */ comments
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)

		if text == SchemaMarker {
			return true, false // +schema without inline
		}
		if text == SchemaMarker+":inline" || strings.HasPrefix(text, SchemaMarker+":inline ") {
			return true, true // +schema:inline
		}
		if strings.HasPrefix(text, SchemaMarker+" ") {
			return true, false // +schema with description
		}
	}
	return false, false
}

// parseStruct parses a struct type specification.
func (p *Parser) parseStruct(typeSpec *ast.TypeSpec, structType *ast.StructType, packageName, filePath string, doc *ast.CommentGroup) StructInfo {
	info := StructInfo{
		Name:     typeSpec.Name.Name,
		Package:  packageName,
		FilePath: filePath,
		Doc:      extractStructDoc(doc, typeSpec.Doc),
	}

	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			fieldInfos := p.parseField(field, p.nameTag)
			for _, fi := range fieldInfos {
				// Skip fields marked with "-" in the tag
				if fi.PropertyName == "-" {
					continue
				}
				info.Fields = append(info.Fields, fi)
			}
		}
	}

	return info
}

// extractStructDoc extracts documentation for a struct.
func extractStructDoc(groupDoc, typeDoc *ast.CommentGroup) string {
	// Prefer type-level doc
	if typeDoc != nil {
		return extractCommentText(typeDoc)
	}
	// Fall back to declaration-level doc
	if groupDoc != nil {
		return extractCommentText(groupDoc)
	}
	return ""
}

// extractCommentText extracts text from a comment group.
func extractCommentText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}

	var lines []string
	for _, c := range cg.List {
		text := c.Text
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
		// Skip empty lines, go directives, and schema markers
		if text == "" || strings.HasPrefix(text, "go:") {
			continue
		}
		if text == SchemaMarker || strings.HasPrefix(text, SchemaMarker+" ") {
			continue
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, " ")
}

// parseTypeExpr converts an AST type expression to TypeInfo.
func (p *Parser) parseTypeExpr(expr ast.Expr) TypeInfo {
	switch t := expr.(type) {
	case *ast.Ident:
		return p.parseIdent(t)

	case *ast.SelectorExpr:
		return p.parseSelectorExpr(t)

	case *ast.StarExpr:
		elemType := p.parseTypeExpr(t.X)
		return TypeInfo{
			Kind:      TypeKindPointer,
			Name:      "*" + elemType.Name,
			IsPointer: true,
			ElemType:  &elemType,
		}

	case *ast.ArrayType:
		elemType := p.parseTypeExpr(t.Elt)
		if t.Len == nil {
			// Slice
			return TypeInfo{
				Kind:     TypeKindSlice,
				Name:     "[]" + elemType.Name,
				ElemType: &elemType,
			}
		}
		// Array
		return TypeInfo{
			Kind:     TypeKindArray,
			Name:     fmt.Sprintf("[...]%s", elemType.Name),
			ElemType: &elemType,
		}

	case *ast.MapType:
		keyType := p.parseTypeExpr(t.Key)
		valueType := p.parseTypeExpr(t.Value)
		return TypeInfo{
			Kind:     TypeKindMap,
			Name:     fmt.Sprintf("map[%s]%s", keyType.Name, valueType.Name),
			KeyType:  &keyType,
			ElemType: &valueType,
		}

	case *ast.InterfaceType:
		return TypeInfo{
			Kind: TypeKindInterface,
			Name: "interface{}",
		}

	case *ast.StructType:
		// Anonymous struct
		return TypeInfo{
			Kind: TypeKindStruct,
			Name: "struct{}",
		}

	default:
		return TypeInfo{
			Kind: TypeKindUnknown,
			Name: "unknown",
		}
	}
}

// parseIdent parses an identifier type.
func (p *Parser) parseIdent(ident *ast.Ident) TypeInfo {
	name := ident.Name

	// Check for primitives
	switch name {
	case "string":
		return TypeInfo{Kind: TypeKindPrimitive, Name: name}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"byte", "rune":
		return TypeInfo{Kind: TypeKindPrimitive, Name: name}
	case "float32", "float64":
		return TypeInfo{Kind: TypeKindPrimitive, Name: name}
	case "bool":
		return TypeInfo{Kind: TypeKindPrimitive, Name: name}
	case "any":
		return TypeInfo{Kind: TypeKindInterface, Name: name}
	default:
		// Check type registry for aliases (e.g., type MyEnum string)
		if decl, ok := p.typeRegistry[name]; ok {
			return TypeInfo{
				Kind:           TypeKindAlias,
				Name:           name,
				IsExported:     ast.IsExported(name),
				UnderlyingKind: decl.UnderlyingKind,
				UnderlyingName: decl.UnderlyingName,
			}
		}

		// Named type (struct reference)
		return TypeInfo{
			Kind:       TypeKindStruct,
			Name:       name,
			IsExported: ast.IsExported(name),
		}
	}
}

// parseSelectorExpr parses a selector expression (e.g., time.Time).
func (p *Parser) parseSelectorExpr(sel *ast.SelectorExpr) TypeInfo {
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return TypeInfo{Kind: TypeKindUnknown, Name: "unknown"}
	}

	pkgName := pkgIdent.Name
	typeName := sel.Sel.Name
	fullName := pkgName + "." + typeName

	// Special case for time.Time
	if pkgName == "time" && typeName == "Time" {
		return TypeInfo{
			Kind:        TypeKindTime,
			Name:        fullName,
			PackageName: pkgName,
		}
	}

	// Special case for time.Duration
	if pkgName == "time" && typeName == "Duration" {
		return TypeInfo{
			Kind:        TypeKindDuration,
			Name:        fullName,
			PackageName: pkgName,
		}
	}

	// External package type
	return TypeInfo{
		Kind:        TypeKindStruct,
		Name:        fullName,
		PackageName: pkgName,
		IsExported:  ast.IsExported(typeName),
	}
}

// FindStructByName finds a specific exported struct by name without requiring the +schema annotation.
// This is used to resolve referenced types that aren't explicitly annotated.
func (p *Parser) FindStructByName(path string, name string, recursive bool) (*StructInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat path %s: %w", path, err)
	}

	if recursive && info.IsDir() {
		return p.findStructInDirRecursive(path, name)
	}

	if info.IsDir() {
		return p.findStructInDir(path, name)
	}
	return p.findStructInFile(path, name)
}

// findStructInDirRecursive recursively searches for a struct by name.
func (p *Parser) findStructInDirRecursive(root string, name string) (*StructInfo, error) {
	var result *StructInfo

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}

		found, err := p.findStructInDir(path, name)
		if err != nil {
			return nil // Continue searching other directories
		}
		if found != nil {
			result = found
			return filepath.SkipAll // Found it, stop searching
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, fmt.Errorf("walk directory %s: %w", root, err)
	}

	return result, nil
}

// findStructInDir searches for a struct by name in a single directory.
func (p *Parser) findStructInDir(dir string, name string) (*StructInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		found, err := p.findStructInFile(filePath, name)
		if err != nil {
			continue
		}
		if found != nil {
			return found, nil
		}
	}

	return nil, nil
}

// findStructInFile searches for a struct by name in a single file.
func (p *Parser) findStructInFile(filePath string, name string) (*StructInfo, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	file, err := parser.ParseFile(p.fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %w", filePath, err)
	}

	packageName := file.Name.Name

	// Extract type declarations for registry
	p.extractTypeDecls(file)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Only match exported types with the target name
			if !typeSpec.Name.IsExported() {
				continue
			}
			if typeSpec.Name.Name != name {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Parse the struct without requiring +schema annotation
			structInfo := p.parseStruct(typeSpec, structType, packageName, filePath, genDecl.Doc)
			return &structInfo, nil
		}
	}

	return nil, nil
}
