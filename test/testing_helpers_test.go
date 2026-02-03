package annot8_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	annot8 "github.com/AxelTahmid/annot8"
)

// NewTestSchemaGenerator returns a fresh SchemaGenerator backed by a newly built TypeIndex.
func NewTestSchemaGenerator() *annot8.SchemaGenerator {
	idx := annot8.BuildTypeIndex()
	return annot8.NewSchemaGenerator(idx)
}

// NewTestGenerator returns a Generator configured with the shared TypeIndex.
func NewTestGenerator() *annot8.Generator {
	idx := annot8.BuildTypeIndex()
	return annot8.NewGeneratorWithCache(idx)
}

// AssertEqual fails the test if expected != actual.
func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

// AssertDeepEqual fails the test if expected and actual are not deeply equal.
func AssertDeepEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("mismatch:\nexpected %#v\nactual   %#v", expected, actual)
	}
}

// AssertNoError fails the test if err is non-nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertJSONEqual fails if two JSON byte slices are not equivalent.
func AssertJSONEqual(t *testing.T, want, got []byte) {
	t.Helper()
	var a, b interface{}
	if err := json.Unmarshal(want, &a); err != nil {
		t.Fatalf("invalid want JSON: %v", err)
	}
	if err := json.Unmarshal(got, &b); err != nil {
		t.Fatalf("invalid got JSON: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("JSON mismatch:\nwant %s\ngot  %s", want, got)
	}
}

// Request creates an HTTP request against handler and returns a recorder.
func Request(handler http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// FindSchemaBySuffix returns the schema whose key ends with suffix or fails the test if not found.
func FindSchemaBySuffix(t *testing.T, schemas map[string]annot8.Schema, suffix string) annot8.Schema {
	t.Helper()
	for name, schema := range schemas {
		if strings.HasSuffix(name, suffix) || name == strings.TrimPrefix(suffix, ".") {
			return schema
		}
	}
	t.Fatalf("expected schema ending with %s, got %v", suffix, schemas)
	return annot8.Schema{}
}

// HasSchemaWithSuffix reports whether schemas contains a key ending with suffix (or exact match).
func HasSchemaWithSuffix(schemas map[string]annot8.Schema, suffix string) bool {
	for name := range schemas {
		if strings.HasSuffix(name, suffix) || name == strings.TrimPrefix(suffix, ".") {
			return true
		}
	}
	return false
}
