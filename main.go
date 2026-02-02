// json-schema-gen generates JSON Schema files from Go structs with
// go-playground/validator tags.
//
// Usage:
//
//	json-schema-gen --output-dir schemas [paths...]
//
// Example go:generate directive:
//
//	//go:generate json-schema-gen --output-dir schemas
package main

import (
	"fmt"
	"os"

	"github.com/ron96g/json-schema-gen/internal/cli"
	"github.com/ron96g/json-schema-gen/internal/generator"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := cli.Parse()
	if err != nil {
		return err
	}

	genCfg := generator.Config{
		OutputDir: cfg.OutputDir,
		NameTag:   cfg.NameTag,
		SchemaID:  cfg.SchemaID,
		Recursive: cfg.Recursive,
	}

	gen := generator.NewGenerator(genCfg)
	return gen.GenerateFromPaths(cfg.Paths)
}
