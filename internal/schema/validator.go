package schema

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/ron96g/json-schema-gen/internal/parser"
)

// ValidatorMapper maps go-playground/validator tags to JSON Schema constraints.
type ValidatorMapper struct{}

// NewValidatorMapper creates a new ValidatorMapper.
func NewValidatorMapper() *ValidatorMapper {
	return &ValidatorMapper{}
}

// ApplyValidation applies validator tag constraints to a JSON Schema.
func (m *ValidatorMapper) ApplyValidation(schema *jsonschema.Schema, field parser.FieldInfo) (isRequired bool) {
	validateTag, ok := field.Tags["validate"]
	if !ok {
		return false
	}

	rules := parseValidateTag(validateTag)

	// Check for dive - split rules into array-level and item-level
	diveIdx := -1
	for i, rule := range rules {
		if rule.Name == "dive" {
			diveIdx = i
			break
		}
	}

	// If dive found and schema is array, apply item-level rules to items
	if diveIdx >= 0 && schema.Type == "array" && schema.Items != nil {
		// Apply rules after dive to items
		itemRules := rules[diveIdx+1:]
		m.applyRulesToSchema(schema.Items, itemRules)
		// Apply rules before dive to array
		rules = rules[:diveIdx]
	}

	isRequired = m.applyRulesToSchema(schema, rules)
	return isRequired
}

// applyRulesToSchema applies validation rules to a schema.
func (m *ValidatorMapper) applyRulesToSchema(schema *jsonschema.Schema, rules []ValidationRule) (isRequired bool) {
	isString := schema.Type == "string"
	isNumeric := schema.Type == "integer" || schema.Type == "number"

	for _, rule := range rules {
		switch rule.Name {
		case "required":
			isRequired = true

		case "omitempty":
			// Not required

		case "min":
			if val, err := strconv.ParseFloat(rule.Param, 64); err == nil {
				if isString {
					minLen := uint64(val)
					schema.MinLength = &minLen
				} else if isNumeric {
					schema.Minimum = json.Number(rule.Param)
				}
			}

		case "max":
			if val, err := strconv.ParseFloat(rule.Param, 64); err == nil {
				if isString {
					maxLen := uint64(val)
					schema.MaxLength = &maxLen
				} else if isNumeric {
					schema.Maximum = json.Number(rule.Param)
				}
			}

		case "len":
			if val, err := strconv.ParseUint(rule.Param, 10, 64); err == nil {
				if isString {
					schema.MinLength = &val
					schema.MaxLength = &val
				}
			}

		case "gte":
			if _, err := strconv.ParseFloat(rule.Param, 64); err == nil {
				if isNumeric {
					schema.Minimum = json.Number(rule.Param)
				}
			}

		case "lte":
			if _, err := strconv.ParseFloat(rule.Param, 64); err == nil {
				if isNumeric {
					schema.Maximum = json.Number(rule.Param)
				}
			}

		case "gt":
			if _, err := strconv.ParseFloat(rule.Param, 64); err == nil {
				if isNumeric {
					schema.ExclusiveMinimum = json.Number(rule.Param)
				}
			}

		case "lt":
			if _, err := strconv.ParseFloat(rule.Param, 64); err == nil {
				if isNumeric {
					schema.ExclusiveMaximum = json.Number(rule.Param)
				}
			}

		case "email":
			schema.Format = "email"

		case "url", "uri", "http_url":
			schema.Format = "uri"

		case "uuid", "uuid3", "uuid4", "uuid5":
			schema.Format = "uuid"

		case "ipv4":
			schema.Format = "ipv4"

		case "ipv6":
			schema.Format = "ipv6"

		case "ip":
			// Could be either, use generic format
			schema.Format = "ip"

		case "datetime":
			schema.Format = "date-time"

		case "date":
			// Custom format for date without time
			schema.Format = "date"

		case "oneof":
			// Parse enum values
			values := strings.Fields(rule.Param)
			if len(values) > 0 {
				enums := make([]any, len(values))
				for i, v := range values {
					enums[i] = v
				}
				schema.Enum = enums
			}

		case "alpha":
			schema.Pattern = "^[a-zA-Z]+$"

		case "alphanum":
			schema.Pattern = "^[a-zA-Z0-9]+$"

		case "alphanumunicode":
			schema.Pattern = "^[\\p{L}\\p{N}]+$"

		case "alphaunicode":
			schema.Pattern = "^\\p{L}+$"

		case "numeric":
			schema.Pattern = "^[0-9]+$"

		case "hexadecimal":
			schema.Pattern = "^[0-9a-fA-F]+$"

		case "lowercase":
			schema.Pattern = "^[a-z]+$"

		case "uppercase":
			schema.Pattern = "^[A-Z]+$"

		case "contains":
			if rule.Param != "" {
				schema.Pattern = regexp.QuoteMeta(rule.Param)
			}

		case "startswith":
			if rule.Param != "" {
				schema.Pattern = "^" + regexp.QuoteMeta(rule.Param)
			}

		case "endswith":
			if rule.Param != "" {
				schema.Pattern = regexp.QuoteMeta(rule.Param) + "$"
			}

		// Array validators
		case "dive":
			// This is handled in schema building for arrays
			// The validators after dive apply to array elements

		// Additional common validators
		case "hostname":
			schema.Format = "hostname"

		case "fqdn":
			schema.Format = "hostname"

		case "json":
			// JSON string

		case "base64":
			// Base64 encoded string
			schema.ContentEncoding = "base64"

		case "ascii":
			schema.Pattern = "^[\\x00-\\x7F]*$"
		}
	}

	return isRequired
}

// ValidationRule represents a parsed validation rule.
type ValidationRule struct {
	Name  string
	Param string
}

// parseValidateTag parses a validate tag into individual rules.
func parseValidateTag(tag string) []ValidationRule {
	var rules []ValidationRule

	// Split by comma, but handle complex cases
	parts := splitValidateTag(tag)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		rule := ValidationRule{}

		// Check for parameter
		if idx := strings.Index(part, "="); idx != -1 {
			rule.Name = part[:idx]
			rule.Param = part[idx+1:]
		} else {
			rule.Name = part
		}

		rules = append(rules, rule)
	}

	return rules
}

// splitValidateTag splits a validate tag, respecting nested structures.
func splitValidateTag(tag string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, ch := range tag {
		switch ch {
		case '[':
			depth++
			current.WriteRune(ch)
		case ']':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
