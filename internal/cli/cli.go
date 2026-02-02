// Package cli handles command-line argument parsing.
package cli

import (
	"flag"
	"fmt"
	"os"
)

// Config holds CLI configuration.
type Config struct {
	OutputDir string   // Output directory for schema files
	NameTag   string   // Tag for property names (json, yaml, etc.)
	SchemaID  string   // Base URL for $id field
	Paths     []string // Input paths (files or directories)
	Recursive bool     // Recursively scan directories for packages
}

// Parse parses command-line arguments and returns configuration.
func Parse() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.OutputDir, "output-dir", "", "Output directory for schema files (required)")
	flag.StringVar(&cfg.NameTag, "tag", "json", "Tag for property names (json/yaml/mapstructure)")
	flag.StringVar(&cfg.SchemaID, "schema-id", "", "Base URL for $id field")
	flag.BoolVar(&cfg.Recursive, "recursive", false, "Recursively scan directories (requires // +schema annotation)")
	flag.BoolVar(&cfg.Recursive, "r", false, "Recursively scan directories (shorthand for --recursive)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json-schema-gen [flags] [paths...]\n\n")
		fmt.Fprintf(os.Stderr, "Generates JSON Schema files from Go structs with go-playground/validator tags.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  json-schema-gen --output-dir schemas ./models/\n")
		fmt.Fprintf(os.Stderr, "  json-schema-gen --output-dir schemas --tag yaml ./api/types.go\n")
		fmt.Fprintf(os.Stderr, "  json-schema-gen --output-dir schemas --schema-id https://example.com/schemas .\n")
		fmt.Fprintf(os.Stderr, "  json-schema-gen --output-dir schemas --recursive .  # scan all subdirs\n")
		fmt.Fprintf(os.Stderr, "\nAnnotation:\n")
		fmt.Fprintf(os.Stderr, "  In recursive mode, only structs with // +schema annotation are processed.\n")
	}

	flag.Parse()

	// Validate required flags
	if cfg.OutputDir == "" {
		return nil, fmt.Errorf("--output-dir is required")
	}

	// Get input paths from positional arguments
	cfg.Paths = flag.Args()
	if len(cfg.Paths) == 0 {
		// Default to current directory
		cfg.Paths = []string{"."}
	}

	// Validate tag
	validTags := map[string]bool{"json": true, "yaml": true, "mapstructure": true, "xml": true}
	if !validTags[cfg.NameTag] {
		return nil, fmt.Errorf("invalid tag %q: must be one of json, yaml, mapstructure, xml", cfg.NameTag)
	}

	return cfg, nil
}
