package annot8fixtures_test

import "testing"

func TestSchemaGenerator_IgnoresJSONDashFields(t *testing.T) {
	t.Parallel()

	sg := NewTestSchemaGenerator()
	_ = sg.GenerateSchema("TestJSONIgnoredField")
	schema := FindSchemaBySuffix(t, sg.GetSchemas(), ".TestJSONIgnoredField")

	if _, ok := schema.Properties["err"]; ok {
		t.Fatalf("expected json:\"-\" field to be omitted from schema properties, got %v", schema.Properties)
	}
	if _, ok := schema.Properties["visible"]; !ok {
		t.Fatalf("expected visible property to remain present, got %v", schema.Properties)
	}
	AssertDeepEqual(t, []string{"visible"}, schema.Required)
}
