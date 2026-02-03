// Package annot8 defines helpers for OpenAPI schema generation from Go types.
package annot8

import (
	"strings"
)

// isBasicType returns true if the Go type name denotes a primitive, array, pointer or map.
// This fast-path is used to decide whether to generate a basic or complex schema.
func isBasicType(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"string", "bool":
		return true
	}
	if strings.HasPrefix(typeName, "[]") || strings.HasPrefix(typeName, "*") || strings.HasPrefix(typeName, "map[") {
		return true
	}
	return false
}

// generateBasicTypeSchema returns a Schema for basic Go types (primitives, slices, pointers).
// It handles arrays and pointers by delegating to GenerateSchema for element types.
func (sg *SchemaGenerator) generateBasicTypeSchema(typeName string) *Schema {
	if strings.HasPrefix(typeName, "[]") {
		elem := strings.TrimPrefix(typeName, "[]")
		return &Schema{Type: "array", Items: sg.GenerateSchema(elem)}
	}
	if strings.HasPrefix(typeName, "*") {
		// Try to see if the pointer type is known externally first (e.g. *time.Time)
		qualified := sg.getQualifiedTypeName(typeName)
		if sg.typeIndex != nil {
			if schema, ok := sg.typeIndex.externalKnownTypes[qualified]; ok {
				return schema
			}
		}

		clean := strings.TrimPrefix(typeName, "*")
		// For basic primitives, use the new 3.1 multi-type array
		if !strings.Contains(clean, ".") && isBasicType(clean) && !strings.HasPrefix(clean, "[]") && !strings.HasPrefix(clean, "map[") {
			underlying := mapGoTypeToOpenAPI(clean)
			return &Schema{
				Type: []string{underlying, "null"},
			}
		}

		// For complex types or slices/maps, use anyOf to avoid type conflicts
		underlying := sg.GenerateSchema(clean)
		return &Schema{
			AnyOf: []*Schema{
				underlying,
				{Type: "null"},
			},
		}
	}
	// Fallback to mapping
	openapiType := mapGoTypeToOpenAPI(typeName)
	return &Schema{Type: openapiType, Description: "basic Go type"}
}

// mapGoTypeToOpenAPI maps a Go type name to the corresponding OpenAPI primitive type.
// Integer and unsigned integer kinds map to "integer", floats to "number", bool to "boolean", and string to "string".
// Other types default to "object".
func mapGoTypeToOpenAPI(typeName string) string {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "string":
		return "string"
	default:
		return "object"
	}
}
