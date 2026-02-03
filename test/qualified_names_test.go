package annot8_test

import (
	"strings"
	"testing"

	"github.com/AxelTahmid/annot8"
)

// TestQualifiedNaming_Internal tests internal types get qualified names
func TestQualifiedNaming_Internal(t *testing.T) {
	gen := NewTestSchemaGenerator()

	// Use an existing type from the annot8 package
	schema := gen.GenerateSchema("Schema")
	if schema == nil || schema.Ref == "" {
		t.Fatal("expected schema reference for Schema")
	}

	// The reference should use qualified name
	expectedRef := "#/components/schemas/annot8.Schema"
	if schema.Ref != expectedRef {
		t.Errorf("expected ref %s, got %s", expectedRef, schema.Ref)
	}

	// Check that the schema is stored under the qualified name
	schemas := gen.GetSchemas()
	if !HasSchemaWithSuffix(schemas, ".Schema") {
		t.Error("schema should be stored under qualified name 'annot8.Schema'")
	}
}

// TestQualifiedNaming_External tests external types
func TestQualifiedNaming_External(t *testing.T) {
	gen := NewTestSchemaGenerator()

	// Test with a known external type
	schema := gen.GenerateSchema("time.Time")
	if schema == nil {
		t.Fatal("expected schema for time.Time")
	}

	if schema.Type != "string" || schema.Format != "date-time" {
		t.Errorf("time.Time should map to string/date-time, got type=%q format=%q", schema.Type, schema.Format)
	}
}

// TestQualifiedNaming_NoDuplicates tests no duplicate schemas
func TestQualifiedNaming_NoDuplicates(t *testing.T) {
	gen := NewTestSchemaGenerator()

	// Generate schema for same type multiple times using existing type
	schema1 := gen.GenerateSchema("Schema")
	schema2 := gen.GenerateSchema("Schema")
	schema3 := gen.GenerateSchema("annot8.Schema") // explicit qualified name

	// All should return the same reference
	if schema1.Ref != schema2.Ref || schema2.Ref != schema3.Ref {
		t.Error("multiple calls for same type should return same reference")
	}

	// Should only have one schema stored
	schemas := gen.GetSchemas()
	count := 0
	for name := range schemas {
		if strings.HasSuffix(name, ".Schema") || name == "Schema" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 schema for Schema, got %d", count)
	}
}

// TestQualifiedNaming_Nested tests nested types use qualified names
func TestQualifiedNaming_Nested(t *testing.T) {
	gen := NewTestSchemaGenerator()

	// Use existing nested types
	schema := gen.GenerateSchema("Components")
	if schema == nil || schema.Ref == "" {
		t.Fatal("expected schema reference for Components")
	}

	// Check that the Components schema exists under qualified name
	schemas := gen.GetSchemas()
	if !HasSchemaWithSuffix(schemas, ".Components") {
		t.Error("Components schema should exist under qualified name")
	}
}

// TestTypeIndexQualifiedLookup tests the new TypeIndex qualified lookup methods
func TestTypeIndexQualifiedLookup(t *testing.T) {
	idx := annot8.BuildTypeIndex()

	t.Run("LookupQualifiedType works", func(t *testing.T) {
		ts := idx.LookupQualifiedType("annot8.Schema")
		if ts == nil {
			t.Error("should find Schema type by qualified name")
		}
	})

	t.Run("GetQualifiedTypeName works", func(t *testing.T) {
		qualifiedName := idx.GetQualifiedTypeName("Schema")
		if qualifiedName != "annot8.Schema" {
			t.Errorf("expected 'annot8.Schema', got '%s'", qualifiedName)
		}

		// Already qualified name should be returned as-is
		alreadyQualified := idx.GetQualifiedTypeName("sqlc.User")
		if alreadyQualified != "sqlc.User" {
			t.Errorf("expected 'sqlc.User', got '%s'", alreadyQualified)
		}
	})

}
