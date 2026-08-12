package annot8fixtures_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AxelTahmid/annot8"
)

// @Summary Multi success endpoint
// @Tags test
// @Success 200 {object} map[string]string "ok"
// @Success 207 {object} map[string]string "partial"
func multiSuccessHandler(w http.ResponseWriter, r *http.Request) {}

func TestGenerateSpec_OperationIDCollectionVsResource(t *testing.T) {
	r := chi.NewRouter()
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	r.Get("/api/v1/menu/category/", stub)
	r.Get("/api/v1/menu/category/{id}", stub)

	spec := annot8.NewGenerator().GenerateSpec(r, annot8.Config{
		Title:   "OperationID Test",
		Version: "1.0.0",
	})

	collection := spec.Paths["/api/v1/menu/category"].Get
	if collection == nil {
		t.Fatal("expected GET operation for /api/v1/menu/category")
	}

	resource := spec.Paths["/api/v1/menu/category/{id}"].Get
	if resource == nil {
		t.Fatal("expected GET operation for /api/v1/menu/category/{id}")
	}

	if collection.OperationID != "getApiV1MenuCategory" {
		t.Fatalf("unexpected collection operationId: %q", collection.OperationID)
	}
	if resource.OperationID != "getApiV1MenuCategoryById" {
		t.Fatalf("unexpected resource operationId: %q", resource.OperationID)
	}
	if collection.OperationID == resource.OperationID {
		t.Fatalf("operationIds must be unique, both were %q", collection.OperationID)
	}
}

func TestGenerateSpec_PathNormalizationAndOperationIDSanitizing(t *testing.T) {
	r := chi.NewRouter()
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	r.Get("/api/v1/menu/{id:[0-9]+}/", stub)
	r.Post("/api/v1/user/tax/tax-groups/{id}/mark-paid", stub)

	spec := annot8.NewGenerator().GenerateSpec(r, annot8.Config{
		Title:   "Path Normalization Test",
		Version: "1.0.0",
	})

	normalized := spec.Paths["/api/v1/menu/{id}"].Get
	if normalized == nil {
		t.Fatal("expected normalized path key /api/v1/menu/{id}")
	}
	if _, exists := spec.Paths["/api/v1/menu/{id}/"]; exists {
		t.Fatal("unexpected trailing-slash path key /api/v1/menu/{id}/")
	}

	op := spec.Paths["/api/v1/user/tax/tax-groups/{id}/mark-paid"].Post
	if op == nil {
		t.Fatal("expected POST operation for /api/v1/user/tax/tax-groups/{id}/mark-paid")
	}
	if op.OperationID != "postApiV1UserTaxTaxGroupsByIdMarkPaid" {
		t.Fatalf("unexpected operationId for hyphenated path: %q", op.OperationID)
	}
}

func TestGenerateSpec_MultipleSuccessResponsesArePreserved(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/multi-success", http.HandlerFunc(multiSuccessHandler))

	spec := annot8.NewGenerator().GenerateSpec(r, annot8.Config{
		Title:   "Multi Success Test",
		Version: "1.0.0",
	})

	op := spec.Paths["/multi-success"].Get
	if op == nil {
		t.Fatal("expected GET operation for /multi-success")
	}
	if _, ok := op.Responses["200"]; !ok {
		t.Fatalf("expected 200 response, got %+v", op.Responses)
	}
	if _, ok := op.Responses["207"]; !ok {
		t.Fatalf("expected 207 response, got %+v", op.Responses)
	}
}

func TestGenerateSpec_PostWithoutBodyParamDoesNotEmitRequestBody(t *testing.T) {
	r := chi.NewRouter()
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	r.Post("/no-body", stub)

	spec := annot8.NewGenerator().GenerateSpec(r, annot8.Config{
		Title:   "No Body Test",
		Version: "1.0.0",
	})

	op := spec.Paths["/no-body"].Post
	if op == nil {
		t.Fatal("expected POST operation for /no-body")
	}
	if op.RequestBody != nil {
		t.Fatalf("expected no requestBody for unannotated POST, got %+v", op.RequestBody)
	}
}
