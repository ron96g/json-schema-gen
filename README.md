# json-schema-gen

Generate JSON Schema files from Go structs with [go-playground/validator](https://github.com/go-playground/validator) tags.

## Installation

```bash
go install github.com/ron96g/json-schema-gen@latest
```

## Usage

```bash
json-schema-gen --output-dir <dir> [flags] [paths...]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--output-dir` | (required) | Output directory for schema files |
| `--tag` | `json` | Tag for property names (`json`, `yaml`, `mapstructure`, `xml`) |
| `--schema-id` | | Base URL for `$id` field |
| `--recursive`, `-r` | `false` | Recursively scan directories (requires `// +schema` annotation) |

## Quick Start

### 1. Define your Go structs

```go
// +schema
type User struct {
    ID      string  `json:"id" validate:"required,uuid"`
    Email   string  `json:"email" validate:"required,email"`
    Age     int     `json:"age" validate:"gte=0,lte=150"`
    Name    string  `json:"name" validate:"required,min=1,max=100"`
    Address Address `json:"address"`
}

// +schema
type Address struct {
    Street  string `json:"street" validate:"required"`
    City    string `json:"city" validate:"required"`
    ZipCode string `json:"zip_code" validate:"required,numeric,len=5"`
    Country string `json:"country" validate:"required,len=2,uppercase"`
}
```

### 2. Generate schemas

```bash
json-schema-gen --output-dir schemas ./models/
```

### 3. Use the generated schema

The generated `user.schema.json` can be used for YAML/JSON validation:

```yaml
# yaml-language-server: $schema=./schemas/user.schema.json
id: "user-001"
email: "max.mustermann@example.com"
name: Max Mustermann
address:
  street: Musterstraße 42
  city: Berlin
  zip_code: "10115"
  country: DE
roles:
  - guest
age: 28
```

### go:generate

Add the tool as a dependency:

```bash
go get -tool github.com/ron96g/json-schema-gen@latest
```

Then add to your Go file:

```go
//go:generate go tool github.com/ron96g/json-schema-gen --output-dir schemas
```

Run:

```bash
go generate ./...
```

## Supported Validators

Common validator tags are translated to JSON Schema:

| Validator | JSON Schema |
|-----------|-------------|
| `required` | `required` array |
| `email` | `format: email` |
| `uuid` | `format: uuid` |
| `url` | `format: uri` |
| `min=N` | `minLength` (string) / `minimum` (number) |
| `max=N` | `maxLength` (string) / `maximum` (number) |
| `len=N` | `minLength` + `maxLength` |
| `gte=N` | `minimum` |
| `lte=N` | `maximum` |
| `oneof=a b c` | `enum: [a, b, c]` |

## Example

See [`examples/simple-go-mod`](examples/simple-go-mod) for a complete working example.
