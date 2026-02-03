package annot8_test

import (
	"testing"

	annot8 "github.com/AxelTahmid/annot8"
)

func TestSchemaGenerator_PrimitiveAndCollectionTypes(t *testing.T) {
	t.Parallel()

	sg := NewTestSchemaGenerator()
	tests := []struct {
		name     string
		typeName string
		assert   func(*testing.T, *annot8.Schema)
	}{
		{
			name:     "int",
			typeName: "int",
			assert: func(t *testing.T, schema *annot8.Schema) {
				AssertEqual(t, "integer", schema.Type)
				AssertEqual(t, "", schema.Ref)
			},
		},
		{
			name:     "string",
			typeName: "string",
			assert: func(t *testing.T, schema *annot8.Schema) {
				AssertEqual(t, "string", schema.Type)
				AssertEqual(t, "", schema.Ref)
			},
		},
		{
			name:     "bool pointer",
			typeName: "*bool",
			assert: func(t *testing.T, schema *annot8.Schema) {
				AssertDeepEqual(t, []string{"boolean", "null"}, schema.Type)
			},
		},
		{
			name:     "slice",
			typeName: "[]string",
			assert: func(t *testing.T, schema *annot8.Schema) {
				AssertEqual(t, "array", schema.Type)
				if schema.Items == nil || schema.Items.Type != "string" {
					t.Fatalf("expected string array items, got %+v", schema.Items)
				}
			},
		},
		{
			name:     "map",
			typeName: "map[string]int",
			assert: func(t *testing.T, schema *annot8.Schema) {
				AssertEqual(t, "object", schema.Type)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.assert(t, sg.GenerateSchema(tc.typeName))
		})
	}
}

func TestSchemaGenerator_ReferenceEmission(t *testing.T) {
	t.Parallel()

	sg := NewTestSchemaGenerator()
	schema := sg.GenerateSchema("TestSimple")
	if schema.Ref == "" {
		t.Fatalf("expected reference for struct type, got %+v", schema)
	}

	stored := sg.GetSchemas()
	if _, ok := stored["annot8.TestSimple"]; !ok {
		t.Fatalf("expected stored schema for annot8.TestSimple, got %v", stored)
	}
}

func TestSchemaGenerator_StructShapes(t *testing.T) {
	t.Parallel()

	t.Run("simple struct required fields", func(t *testing.T) {
		sg := NewTestSchemaGenerator()
		_ = sg.GenerateSchema("TestSimple")
		schema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestSimple")

		AssertEqual(t, "object", schema.Type)
		AssertDeepEqual(t, []string{"id", "name"}, schema.Required)

		if prop, ok := schema.Properties["id"]; !ok || prop.Type != "integer" {
			t.Fatalf("expected integer property 'id', got %+v", prop)
		}
		if prop, ok := schema.Properties["name"]; !ok || prop.Type != "string" {
			t.Fatalf("expected string property 'name', got %+v", prop)
		}
	})

	t.Run("pointer field omitempty", func(t *testing.T) {
		sg := NewTestSchemaGenerator()
		_ = sg.GenerateSchema("TestWithPointer")
		schema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestWithPointer")

		if len(schema.Required) != 0 {
			t.Fatalf("expected no required fields due to omitempty, got %v", schema.Required)
		}
		if prop, ok := schema.Properties["name"]; !ok {
			t.Fatalf("expected property 'name' to exist")
		} else {
			AssertDeepEqual(t, []string{"string", "null"}, prop.Type)
		}
	})

	t.Run("OpenAPI 3.1 compliance features", func(t *testing.T) {
		sg := NewTestSchemaGenerator()
		_ = sg.GenerateSchema("TestCompliance31")
		schema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestCompliance31")

		count := schema.Properties["count"]
		if count.ExclusiveMinimum == nil || *count.ExclusiveMinimum != 0 {
			t.Errorf("expected ExclusiveMinimum 0, got %v", count.ExclusiveMinimum)
		}
		if count.ExclusiveMaximum == nil || *count.ExclusiveMaximum != 100 {
			t.Errorf("expected ExclusiveMaximum 100, got %v", count.ExclusiveMaximum)
		}

		rate := schema.Properties["rate"]
		if rate.ExclusiveMinimum == nil || *rate.ExclusiveMinimum != 0.5 {
			t.Errorf("expected ExclusiveMinimum 0.5, got %v", rate.ExclusiveMinimum)
		}
	})

	t.Run("type aliases (maps and slices)", func(t *testing.T) {
		sg := NewTestSchemaGenerator()

		// Test map alias
		_ = sg.GenerateSchema("TestAliasMap")
		mapSchema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestAliasMap")
		if mapSchema.Type != "object" {
			t.Errorf("expected object type for TestAliasMap, got %v", mapSchema.Type)
		}
		if mapSchema.AdditionalProperties == nil {
			t.Errorf("expected additionalProperties for TestAliasMap")
		}

		// Test slice alias
		_ = sg.GenerateSchema("TestAliasSlice")
		sliceSchema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestAliasSlice")
		if sliceSchema.Type != "array" {
			t.Errorf("expected array type for TestAliasSlice, got %v", sliceSchema.Type)
		}
		if sliceSchema.Items == nil {
			t.Errorf("expected items for TestAliasSlice")
		}
	})

	t.Run("array field schema", func(t *testing.T) {
		sg := NewTestSchemaGenerator()
		_ = sg.GenerateSchema("TestWithArray")
		container := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestWithArray")

		prop, ok := container.Properties["tags"]
		if !ok || prop.Type != "array" || prop.Items == nil || prop.Items.Type != "string" {
			t.Fatalf("expected array property 'tags', got %+v", prop)
		}
	})

	t.Run("nested references", func(t *testing.T) {
		sg := NewTestSchemaGenerator()
		ref := sg.GenerateSchema("TestNested")
		if ref.Ref == "" {
			t.Fatalf("expected ref for nested struct, got %+v", ref)
		}

		nested := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestNested")
		child, ok := nested.Properties["simple"]
		if !ok || child.Ref == "" {
			t.Fatalf("expected nested property to reference TestSimple, got %+v", child)
		}
	})

	t.Run("qualified nested type", func(t *testing.T) {
		sg := NewTestSchemaGenerator()
		schema := sg.GenerateSchema("TestWithQualified")
		if schema.Ref == "" {
			t.Fatalf("expected ref for TestWithQualified, got %+v", schema)
		}
		stored := sg.GetSchemas()
		if !HasSchemaWithSuffix(stored, ".TestWithQualified") {
			t.Fatalf("expected schema for TestWithQualified, got %v", stored)
		}
	})
}

func TestSchemaGenerator_TagEnhancements(t *testing.T) {
	t.Parallel()

	sg := NewTestSchemaGenerator()
	_ = sg.GenerateSchema("annot8.TagExample")
	schema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TagExample")

	id := schema.Properties["id"]
	AssertEqual(t, "uuid", id.Format)
	if id.Deprecated == nil || *id.Deprecated != true {
		t.Fatalf("expected id to be deprecated, got %v", id.Deprecated)
	}
	AssertEqual(t, "00000000-0000-0000-0000-000000000000", id.Default)

	alias := schema.Properties["alias"]
	AssertEqual(t, "^a.*$", alias.Pattern)
	if alias.MinLength == nil || *alias.MinLength != 2 {
		t.Fatalf("expected alias minLength=2, got %v", alias.MinLength)
	}
	if alias.MaxLength == nil || *alias.MaxLength != 5 {
		t.Fatalf("expected alias maxLength=5, got %v", alias.MaxLength)
	}

	email := schema.Properties["email"]
	AssertEqual(t, "email", email.Format)

	owner := schema.Properties["owner"]
	AssertEqual(t, "uuid", owner.Format)
}
