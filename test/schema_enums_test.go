package annot8_test

import (
	"testing"

	annot8 "github.com/AxelTahmid/annot8"
)

func TestGenerateSchema_EnumType(t *testing.T) {
	sg := NewTestSchemaGenerator()

	result := sg.GenerateSchema("annot8.MyEnum")
	if result.Ref == "" {
		t.Fatalf("expected enum schema to be returned as reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	enumSchema, ok := schemas["annot8.MyEnum"]
	if !ok {
		t.Fatalf("expected enum schema to be stored under 'annot8.MyEnum', got %v", schemas)
	}
	AssertEqual(t, "string", enumSchema.Type)
	AssertDeepEqual(t, []interface{}{"A", "B"}, enumSchema.Enum)
}

func TestGenerateSchema_EnumFieldInStruct(t *testing.T) {
	sg := NewTestSchemaGenerator()

	// Generate schema for a struct containing an enum field
	result := sg.GenerateSchema("annot8.TestWithEnumField")
	if result.Ref == "" {
		t.Fatalf("expected struct schema to be returned as reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	structSchema, ok := schemas["annot8.TestWithEnumField"]
	if !ok {
		t.Fatalf("expected stored schema for TestWithEnumField, got %v", schemas)
	}

	// Verify the enum field references the proper enum schema
	statusProp, ok := structSchema.Properties["status"]
	if !ok {
		t.Fatalf("expected 'status' property in struct, got %+v", structSchema.Properties)
	}

	// CRITICAL: The status property must be a reference, not an inlined enum
	if statusProp.Ref == "" {
		t.Errorf("expected status property to reference enum schema, got: %+v", statusProp)
		if statusProp.Type == "string" && len(statusProp.Enum) > 0 {
			t.Error("ENUM BUG DETECTED: Enum is inlined in struct instead of being referenced!")
		}
	} else {
		expectedRef := "#/components/schemas/annot8.MyEnum"
		if statusProp.Ref != expectedRef {
			t.Errorf("expected reference '%s', got '%s'", expectedRef, statusProp.Ref)
		}
	}

	// Verify the enum schema itself is properly stored
	enumSchema, ok := schemas["annot8.MyEnum"]
	if !ok {
		t.Fatalf("expected enum schema to be stored, got %v", schemas)
	}
	AssertEqual(t, "string", enumSchema.Type)
	AssertDeepEqual(t, []interface{}{"A", "B"}, enumSchema.Enum)
}

// TestGenerateSchema_NullableEnumFromSqlc tests enum handling with sqlc-generated nullable types
// This simulates how enums from pkg/db/sqlc are used in structs
func TestGenerateSchema_NullableEnumFromSqlc(t *testing.T) {
	sg := NewTestSchemaGenerator()

	// Simulate a struct that contains a nullable enum field (like NullBillingModel)
	// The issue might manifest here if the code isn't properly detecting enum types
	result := sg.GenerateSchema("sqlc.Coupon")
	if result.Ref == "" {
		t.Fatalf("expected struct schema reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	couponSchema, ok := schemas["sqlc.Coupon"]
	if !ok {
		t.Fatalf("expected Coupon schema to be stored, got keys: %v", SchemaKeys(schemas))
	}

	t.Logf("Coupon schema properties: %v", PropertyNames(couponSchema.Properties))

	// Check that enum fields reference the proper enum schema
	typeField, ok := couponSchema.Properties["type"]
	if !ok {
		t.Fatalf("expected 'type' property in Coupon struct")
	}
	if typeField.Ref == "" {
		t.Errorf("ENUM BUG: 'type' field should reference DiscountType enum, got %+v", typeField)
	}

	scopeField, ok := couponSchema.Properties["scope"]
	if !ok {
		t.Fatalf("expected 'scope' property in Coupon struct")
	}
	if scopeField.Ref == "" {
		t.Errorf("ENUM BUG: 'scope' field should reference CouponScope enum, got %+v", scopeField)
	}
}

func TestGenerateSchema_NonEnumFallback(t *testing.T) {
	sg := NewTestSchemaGenerator()

	result := sg.GenerateSchema("nonexistent.Type")
	if result.Ref == "" {
		t.Fatalf("expected fallback schema to be stored as reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	fallback, ok := schemas["nonexistent.Type"]
	if !ok {
		t.Fatalf("expected stored schema for nonexistent.Type, got %v", schemas)
	}
	if fallback.Type != "object" || fallback.Description == "" {
		t.Fatalf("expected fallback object schema, got %+v", fallback)
	}
}

// TestGenerateSchema_EnumWithTag tests that struct tags don't corrupt enum references
func TestGenerateSchema_EnumWithTag(t *testing.T) {
	sg := NewTestSchemaGenerator()

	// Add a test struct that has an enum field with OpenAPI tags
	// This tests the bug where applyEnhancedTags modifies referenced schemas
	t.Run("enum field with annot8 tags should remain a reference", func(t *testing.T) {
		result := sg.GenerateSchema("annot8.TestWithEnumField")
		if result.Ref == "" {
			t.Fatalf("expected struct schema reference, got %+v", result)
		}

		schemas := sg.GetSchemas()
		structSchema, ok := schemas["annot8.TestWithEnumField"]
		if !ok {
			t.Fatalf("expected TestWithEnumField schema")
		}

		statusProp := structSchema.Properties["status"]
		if statusProp.Ref == "" {
			t.Fatalf("CRITICAL BUG: enum field should be a reference, got %+v", statusProp)
		}

		// Verify the reference is correct
		if statusProp.Ref != "#/components/schemas/annot8.MyEnum" {
			t.Errorf("expected reference to MyEnum, got %s", statusProp.Ref)
		}

		// IMPORTANT: A schema with $ref should NOT have Type, Enum, or other properties
		// This is per OpenAPI 3.1 spec
		if statusProp.Type != nil {
			t.Errorf("BUG: referenced schema should not have Type set, got: %v", statusProp.Type)
		}
		if len(statusProp.Enum) > 0 {
			t.Errorf("BUG: referenced schema should not have Enum set, got: %v", statusProp.Enum)
		}
	})
}

// Helper functions
func SchemaKeys(schemas map[string]annot8.Schema) []string {
	keys := make([]string, 0, len(schemas))
	for k := range schemas {
		keys = append(keys, k)
	}
	return keys
}

func PropertyNames(props map[string]*annot8.Schema) []string {
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	return names
}

// TestEnumReference_NotCorruptedByTags verifies that struct tags don't modify
// enum reference schemas. Per OpenAPI 3.1 spec, $ref cannot have sibling properties.
func TestEnumReference_NotCorruptedByTags(t *testing.T) {
	sg := NewTestSchemaGenerator()

	// Generate a struct with an enum field that might have tags
	result := sg.GenerateSchema("annot8.TestWithEnumField")
	if result.Ref == "" {
		t.Fatalf("expected struct schema reference")
	}

	schemas := sg.GetSchemas()
	structSchema := schemas["annot8.TestWithEnumField"]

	statusField := structSchema.Properties["status"]

	// CRITICAL: Field must be a reference, not modified by tags
	if statusField.Ref == "" {
		t.Errorf("BUG: Enum field corrupted by tags, got: %+v", statusField)
	}

	// Verify reference is correct
	AssertEqual(t, "#/components/schemas/annot8.MyEnum", statusField.Ref)

	// Per OpenAPI 3.1: A schema object containing a $ref property SHOULD NOT contain
	// any other properties except description, examples, and metadata keywords
	if statusField.Type != nil {
		t.Errorf("BUG: Referenced schema has Type sibling: %v", statusField.Type)
	}
	if len(statusField.Enum) > 0 {
		t.Errorf("BUG: Referenced schema has Enum sibling: %v", statusField.Enum)
	}
}

// TestEnumEdgeCase_NoConstantsFound tests what happens when an enum type has no
// extractable constants. This might be the source of the reported bug!
func TestEnumEdgeCase_NoConstantsFound(t *testing.T) {
	sg := NewTestSchemaGenerator()

	// Try to generate schema for the first field value as if it were being extracted
	// This test checks if maybe the code is falling back to inlining a string with
	// just one value when no enum constants are found

	result := sg.GenerateSchema("annot8.TypeWithoutConstants")
	if result.Ref == "" {
		t.Fatalf("expected reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	schema, ok := schemas["annot8.TypeWithoutConstants"]
	if !ok {
		// If type is not in typeindex, it returns the fallback
		t.Logf("Type not indexed, this is expected for non-existent types")
		return
	}

	// If it IS a string type with Enum values, that's the bug!
	if tStr, ok := schema.Type.(string); ok && tStr == "string" && len(schema.Enum) > 0 {
		t.Logf("WARNING: Found a type with inlined enum: %+v", schema)
	}
}

// TestEnumEdgeCase_RealWorldSqlcExample tests what happens with real sqlc enums
func TestEnumEdgeCase_RealWorldSqlcExample(t *testing.T) {
	sg := NewTestSchemaGenerator()

	// Test the actual DiscountType from sqlc
	result := sg.GenerateSchema("sqlc.DiscountType")
	if result.Ref == "" {
		t.Fatalf("expected enum reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	enumSchema, ok := schemas["sqlc.DiscountType"]
	if !ok {
		t.Fatalf("expected DiscountType schema to be generated")
	}

	AssertEqual(t, "string", enumSchema.Type)
	t.Logf("DiscountType enum values: %v", enumSchema.Enum)

	// Should have exactly the two values defined in models.go
	if len(enumSchema.Enum) != 2 {
		t.Errorf("Expected 2 DiscountType values, got %d: %v", len(enumSchema.Enum), enumSchema.Enum)
	}

	hasPercentage := false
	hasFixed := false
	for _, v := range enumSchema.Enum {
		switch v {
		case "percentage":
			hasPercentage = true
		case "fixed":
			hasFixed = true
		case "":
			t.Error("BUG: Empty string in enum values!")
		}
	}

	if !hasPercentage {
		t.Error("Missing 'percentage' value from DiscountType enum")
	}
	if !hasFixed {
		t.Error("Missing 'fixed' value from DiscountType enum")
	}
}

// TestEnumEdgeCase_ImplicitType tests that enums with implicit type from predecessor
// are correctly extracted
func TestEnumEdgeCase_ImplicitType(t *testing.T) {
	sg := NewTestSchemaGenerator()

	result := sg.GenerateSchema("annot8.StatusEnum")
	if result.Ref == "" {
		t.Fatalf("expected enum reference, got %+v", result)
	}

	schemas := sg.GetSchemas()
	enumSchema, ok := schemas["annot8.StatusEnum"]
	if !ok {
		t.Fatalf("expected StatusEnum schema")
	}

	AssertEqual(t, "string", enumSchema.Type)

	// This is the critical test: does it extract the implicitly-typed constant?
	t.Logf("Extracted enum values: %v", enumSchema.Enum)

	// Should have 2 values at minimum (active, pending)
	// The implicit "inactive" may or may not be extracted depending on const handling
	if len(enumSchema.Enum) < 2 {
		t.Errorf("BUG FOUND: Expected at least 2 enum values (active, pending), got %v", enumSchema.Enum)
	}

	// Check for the explicit values at minimum
	hasActive := false
	hasPending := false
	for _, v := range enumSchema.Enum {
		switch v {
		case "active":
			hasActive = true
		case "pending":
			hasPending = true
		}
	}

	if !hasActive {
		t.Error("Missing 'active' enum value")
	}
	if !hasPending {
		t.Error("Missing 'pending' enum value")
	}
}

// TestEnumEdgeCase_MultipleTypesInBlock tests that const blocks with multiple
// enum types don't get mixed up
func TestEnumEdgeCase_MultipleTypesInBlock(t *testing.T) {
	sg := NewTestSchemaGenerator()

	resultA := sg.GenerateSchema("annot8.TypeA")
	if resultA.Ref == "" {
		t.Fatalf("expected TypeA enum reference, got %+v", resultA)
	}

	resultB := sg.GenerateSchema("annot8.TypeB")
	if resultB.Ref == "" {
		t.Fatalf("expected TypeB enum reference, got %+v", resultB)
	}

	schemas := sg.GetSchemas()

	schemaA, ok := schemas["annot8.TypeA"]
	if !ok {
		t.Fatalf("expected TypeA schema")
	}

	schemaB, ok := schemas["annot8.TypeB"]
	if !ok {
		t.Fatalf("expected TypeB schema")
	}

	// Check TypeA has correct values
	t.Logf("TypeA enum values: %v", schemaA.Enum)
	if len(schemaA.Enum) != 2 {
		t.Errorf("Expected 2 values for TypeA, got %d: %v", len(schemaA.Enum), schemaA.Enum)
	}

	// Check TypeB has correct values
	t.Logf("TypeB enum values: %v", schemaB.Enum)
	if len(schemaB.Enum) != 2 {
		t.Errorf("Expected 2 values for TypeB, got %d: %v", len(schemaB.Enum), schemaB.Enum)
	}

	// Verify they don't overlap
	for _, valA := range schemaA.Enum {
		for _, valB := range schemaB.Enum {
			if valA == valB {
				t.Errorf("BUG: TypeA and TypeB enum values overlap at %v", valA)
			}
		}
	}
}
