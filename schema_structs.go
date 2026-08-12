package annot8

import (
	"go/ast"
	"log/slog"
	"reflect"
	"strings"
)

// convertStructToSchema converts a Go AST struct type into an OpenAPI object schema.
func (sg *SchemaGenerator) convertStructToSchema(structType *ast.StructType) *Schema {
	slog.Debug("[annot8] convertStructToSchema: called")

	var allOf []*Schema
	properties := make(map[string]*Schema)
	var required []string
	sqlcNullValueFieldJSONName, isSQLCNullWrapper := sg.detectSQLCNullWrapper(structType)

	for _, field := range structType.Fields.List {
		// Ensure dependent schemas generated for the field type
		switch t := field.Type.(type) {
		case *ast.Ident:
			if t.Obj != nil && t.Obj.Kind == ast.Typ {
				qualified := sg.getQualifiedTypeName(t.Name)
				_ = sg.GenerateSchema(qualified)
			}
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Typ {
				qualified := sg.getQualifiedTypeName(ident.Name)
				_ = sg.GenerateSchema(qualified)
			}
		case *ast.SelectorExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				qualified := ident.Name + "." + t.Sel.Name
				_ = sg.GenerateSchema(qualified)
			}
		}

		if len(field.Names) == 0 {
			// Handle embedded field
			embeddedSchema := sg.convertFieldType(field.Type)
			allOf = append(allOf, embeddedSchema)
			continue
		}

		for _, nameIdent := range field.Names {
			fieldName := nameIdent.Name
			if !ast.IsExported(fieldName) {
				continue // skip unexported
			}

			// Determine JSON property name
			jsonName := fieldName
			if field.Tag != nil {
				tag := strings.Trim(field.Tag.Value, "`")
				jsonTag := extractJSONTag(tag)
				if jsonTag == "-" {
					continue
				}
				if jsonTag != "" {
					jsonName = jsonTag
				}
			}

			// Convert field type
			fieldSchema := sg.convertFieldType(field.Type)
			if isSQLCNullWrapper && jsonName == sqlcNullValueFieldJSONName {
				fieldSchema = wrapSQLCNullWrapperValueSchema(fieldSchema)
			}

			// Apply struct tag enhancements ONLY if not a reference schema
			// References should not have sibling properties per OpenAPI 3.1 spec
			if field.Tag != nil && fieldSchema.Ref == "" {
				tag := strings.Trim(field.Tag.Value, "`")
				sg.applyEnhancedTags(fieldSchema, tag)
			}

			properties[jsonName] = fieldSchema

			// Determine required fields
			if isSQLCNullWrapper && jsonName == sqlcNullValueFieldJSONName {
				continue
			}
			if !isPointerType(field.Type) && !hasOmitEmpty(field.Tag) {
				required = append(required, jsonName)
			}
		}
	}

	if len(allOf) == 0 {
		return &Schema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		}
	}

	// if we have local properties, add as anonymous object to allOf
	if len(properties) > 0 {
		allOf = append(allOf, &Schema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		})
	}

	return &Schema{
		AllOf: allOf,
	}
}

// convertFieldType inspects a Go AST expression and returns its OpenAPI schema representation.
// It handles identifiers, pointers, arrays, selectors, maps, and empty interfaces.
func (sg *SchemaGenerator) convertFieldType(expr ast.Expr) *Schema {
	slog.Debug("[annot8] convertFieldType: called")

	switch t := expr.(type) {
	case *ast.Ident:
		// Basic Go types
		basicType, basicFormat := mapGoTypeToOpenAPI(t.Name)
		if basicType != "object" {
			schema := &Schema{Type: basicType}
			if basicFormat != "" {
				schema.Format = basicFormat
			}
			return schema
		}
		// Custom types
		qualified := sg.getQualifiedTypeName(t.Name)
		return sg.GenerateSchema(qualified)

	case *ast.StarExpr:
		// Pointer types: check for external types first (like *time.Time).
		// Clone: callers decorate field schemas in place (tag enhancement),
		// which must not mutate the shared canonical entry.
		if ident, ok := t.X.(*ast.Ident); ok {
			qualified := "*" + sg.getQualifiedTypeName(ident.Name)
			if sg.typeIndex != nil {
				if schema, ok := sg.typeIndex.externalKnownTypes[qualified]; ok {
					return cloneSchema(schema)
				}
			}
		} else if sel, ok := t.X.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				qualified := "*" + ident.Name + "." + sel.Sel.Name
				if sg.typeIndex != nil {
					if schema, ok := sg.typeIndex.externalKnownTypes[qualified]; ok {
						return cloneSchema(schema)
					}
				}
			}
		}

		// Fallback: Pointer types: wrap with nullability to support OAS 3.1
		underlying := sg.convertFieldType(t.X)
		if underlying.Ref != "" {
			// For references, we use anyOf to avoid type conflicts (e.g. if the ref is a string enum)
			return &Schema{
				AnyOf: []*Schema{
					underlying,
					{Type: "null"},
				},
			}
		}

		if tStr, ok := underlying.Type.(string); ok {
			underlying.Type = []string{tStr, "null"}
		}
		return underlying

	case *ast.ArrayType:
		// Arrays and slices
		elem := sg.convertFieldType(t.Elt)
		return &Schema{Type: "array", Items: elem}

	case *ast.SelectorExpr:
		// Qualified types (e.g., time.Time)
		if ident, ok := t.X.(*ast.Ident); ok {
			qualified := ident.Name + "." + t.Sel.Name
			return sg.GenerateSchema(qualified)
		}

	case *ast.MapType:
		// Maps as object with additionalProperties
		return &Schema{Type: "object", AdditionalProperties: sg.convertFieldType(t.Value)}

	case *ast.InterfaceType:
		// Empty interface as object
		return &Schema{Type: "object"}
	}

	slog.Debug("[annot8] convertFieldType: unknown type, defaulting to object")
	return &Schema{Type: "object"}
}

// isPointerType returns true if the given AST expression represents a pointer type.
func isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// hasOmitEmpty reports whether the field's json tag carries the omitempty or
// omitzero option — i.e. the field may be absent from the payload. Only the
// json tag is consulted: a substring match over the whole tag would also hit
// validator directives like validate:"omitempty,..." and wrongly mark
// JSON-required fields optional.
func hasOmitEmpty(tag *ast.BasicLit) bool {
	if tag == nil {
		return false
	}
	structTag := reflect.StructTag(strings.Trim(tag.Value, "`"))
	jsonTag, ok := structTag.Lookup("json")
	if !ok {
		return false
	}
	options := strings.Split(jsonTag, ",")
	for _, opt := range options[1:] {
		if opt == "omitempty" || opt == "omitzero" {
			return true
		}
	}
	return false
}

func (sg *SchemaGenerator) detectSQLCNullWrapper(structType *ast.StructType) (string, bool) {
	if sg.currentPackage != "sqlc" || structType == nil {
		return "", false
	}

	var valueFieldJSONName string
	hasValueField := false
	hasValidBool := false

	for _, field := range structType.Fields.List {
		if len(field.Names) != 1 {
			continue
		}

		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			continue
		}

		if fieldName == "Valid" && isBoolType(field.Type) {
			hasValidBool = true
			continue
		}

		if hasValueField {
			return "", false
		}

		hasValueField = true
		valueFieldJSONName = fieldName
		if field.Tag != nil {
			tag := strings.Trim(field.Tag.Value, "`")
			if jsonTag := extractJSONTag(tag); jsonTag != "" && jsonTag != "-" {
				valueFieldJSONName = jsonTag
			}
		}
	}

	if !hasValueField || !hasValidBool {
		return "", false
	}

	return valueFieldJSONName, true
}

func isBoolType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "bool"
}

func wrapSQLCNullWrapperValueSchema(fieldSchema *Schema) *Schema {
	if fieldSchema == nil {
		return nil
	}

	anyOf := []*Schema{
		cloneSchema(fieldSchema),
		{Type: "null"},
	}

	if schemaSupportsString(fieldSchema) {
		anyOf = append(anyOf, &Schema{
			Type: "string",
			Enum: []any{""},
		})
	}

	return &Schema{AnyOf: anyOf}
}

func schemaSupportsString(s *Schema) bool {
	if s == nil {
		return false
	}
	if s.Ref != "" {
		return true
	}

	switch t := s.Type.(type) {
	case string:
		return t == "string"
	case []string:
		for _, v := range t {
			if v == "string" {
				return true
			}
		}
	case []any:
		for _, v := range t {
			if str, ok := v.(string); ok && str == "string" {
				return true
			}
		}
	}

	return len(s.Enum) > 0
}
