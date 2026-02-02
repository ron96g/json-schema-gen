package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invopop/jsonschema"
)

// Writer handles writing JSON Schema files to disk.
type Writer struct {
	outputDir string
}

// NewWriter creates a new Writer.
func NewWriter(outputDir string) *Writer {
	return &Writer{
		outputDir: outputDir,
	}
}

// WriteSchema writes a JSON Schema to a file.
func (w *Writer) WriteSchema(typeName string, schema *jsonschema.Schema) error {
	// Ensure output directory exists
	if err := os.MkdirAll(w.outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Generate filename: lowercase typename + .schema.json
	filename := strings.ToLower(typeName) + ".schema.json"
	filepath := filepath.Join(w.outputDir, filename)

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schema: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("Generated: %s\n", filepath)
	return nil
}

// GetSchemaFilename returns the schema filename for a type.
func GetSchemaFilename(typeName string) string {
	return strings.ToLower(typeName) + ".schema.json"
}
