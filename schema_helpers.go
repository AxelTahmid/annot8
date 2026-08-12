package annot8

// hasType checks if the schema includes the specified OpenAPI type.
func hasType(s *Schema, typeName string) bool {
	if s.Type == nil {
		return false
	}
	switch t := s.Type.(type) {
	case string:
		return t == typeName
	case []string:
		for _, v := range t {
			if v == typeName {
				return true
			}
		}
	case []any:
		// External known types are declared with []any{"number", "null"}.
		for _, v := range t {
			if str, ok := v.(string); ok && str == typeName {
				return true
			}
		}
	}
	return false
}
