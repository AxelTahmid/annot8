package annot8fixtures_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AxelTahmid/annot8"
)

type validatedHandler struct{}

// @Summary Health check
// @Tags system
// @Success 200 {object} map[string]string "ok"
func (h *validatedHandler) ok(w http.ResponseWriter, r *http.Request) {}

func (h *validatedHandler) missing(w http.ResponseWriter, r *http.Request) {}

// @Summary Broken annotation
// @Tags system
// @Success 200 {object} map[string]string "ok"
// @Param malformed
func (h *validatedHandler) malformed(w http.ResponseWriter, r *http.Request) {}

func TestValidateAnnotations(t *testing.T) {
	r := chi.NewRouter()
	h := &validatedHandler{}
	r.Get("/validated", http.HandlerFunc(h.ok))
	r.Get("/missing", http.HandlerFunc(h.missing))

	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, annot8.Config{Title: "Validation Test", Version: "1.0.0"})

	violations := annot8.ValidateAnnotations(&spec)
	if len(violations) != 3 {
		t.Fatalf("expected 3 violations for unannotated handler, got %d: %v", len(violations), violations)
	}

	joined := strings.Join(violations, "\n")
	for _, want := range []string{
		"GET /missing: missing @Summary",
		"GET /missing: missing @Tags",
		"GET /missing: missing @Success",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected violation %q in %q", want, joined)
		}
	}
}

func TestValidateRefs(t *testing.T) {
	spec := &annot8.Spec{
		Paths: map[string]annot8.PathItem{
			"/orders": {
				Get: &annot8.Operation{
					Responses: annot8.Responses{
						"200": {
							Description: "ok",
							Content: map[string]annot8.MediaTypeObject{
								"application/json": {
									Schema: &annot8.Schema{Ref: "#/components/schemas/OrderListResponse"},
								},
							},
						},
					},
				},
			},
		},
		Components: &annot8.Components{
			Schemas: map[string]annot8.Schema{},
		},
	}

	violations := annot8.ValidateRefs(spec)
	if len(violations) != 1 {
		t.Fatalf("expected one missing ref violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "#/components/schemas/OrderListResponse") {
		t.Fatalf("unexpected violation: %q", violations[0])
	}
}

func TestValidateAnnotations_ReportsParseErrors(t *testing.T) {
	r := chi.NewRouter()
	h := &validatedHandler{}
	r.Get("/malformed", http.HandlerFunc(h.malformed))

	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, annot8.Config{Title: "Validation Parse Error Test", Version: "1.0.0"})

	violations := annot8.ValidateAnnotations(&spec)
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "annotation parse error") {
		t.Fatalf("expected parse error violation, got %v", violations)
	}
	if !strings.Contains(joined, "invalid @Param annotation") {
		t.Fatalf("expected invalid @Param annotation violation, got %v", violations)
	}
}

func TestValidateOperationIDs(t *testing.T) {
	spec := &annot8.Spec{
		Paths: map[string]annot8.PathItem{
			"/a": {
				Get: &annot8.Operation{
					OperationID: "dupID",
					Responses:   annot8.Responses{"200": {Description: "ok"}},
				},
			},
			"/b": {
				Post: &annot8.Operation{
					OperationID: "dupID",
					Responses:   annot8.Responses{"200": {Description: "ok"}},
				},
			},
			"/c": {
				Delete: &annot8.Operation{
					Responses: annot8.Responses{"200": {Description: "ok"}},
				},
			},
		},
	}

	violations := annot8.ValidateOperationIDs(spec)
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "duplicate operationId") {
		t.Fatalf("expected duplicate operationId violation, got %v", violations)
	}
	if !strings.Contains(joined, "missing operationId") {
		t.Fatalf("expected missing operationId violation, got %v", violations)
	}
}

func TestValidateAmbiguousPaths(t *testing.T) {
	spec := &annot8.Spec{
		Paths: map[string]annot8.PathItem{
			"/api/v1/coupon/{id}": {
				Get: &annot8.Operation{OperationID: "getCouponByID", Responses: annot8.Responses{"200": {Description: "ok"}}},
			},
			"/api/v1/coupon/{couponId}": {
				Get: &annot8.Operation{OperationID: "getCouponByCouponID", Responses: annot8.Responses{"200": {Description: "ok"}}},
			},
		},
	}

	violations := annot8.ValidateAmbiguousPaths(spec)
	if len(violations) != 1 {
		t.Fatalf("expected one ambiguous path violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "/api/v1/coupon/{id}") || !strings.Contains(violations[0], "/api/v1/coupon/{couponId}") {
		t.Fatalf("unexpected ambiguous path violation: %q", violations[0])
	}
}

func TestValidateAmbiguousPaths_ConcreteStaticPathIsAllowed(t *testing.T) {
	spec := &annot8.Spec{
		Paths: map[string]annot8.PathItem{
			"/api/v1/coupon/{id}": {
				Get: &annot8.Operation{OperationID: "getCouponByID", Responses: annot8.Responses{"200": {Description: "ok"}}},
			},
			"/api/v1/coupon/validate": {
				Get: &annot8.Operation{OperationID: "getCouponValidate", Responses: annot8.Responses{"200": {Description: "ok"}}},
			},
		},
	}

	violations := annot8.ValidateAmbiguousPaths(spec)
	if len(violations) != 0 {
		t.Fatalf("expected no ambiguity for concrete-vs-templated path, got %v", violations)
	}
}

func TestValidateRefs_NonSchemaComponents(t *testing.T) {
	spec := &annot8.Spec{
		Paths: map[string]annot8.PathItem{
			"/orders": {
				Post: &annot8.Operation{
					Responses: annot8.Responses{
						"200": {
							Description: "ok",
							Content: map[string]annot8.MediaTypeObject{
								"application/json": {
									Schema: &annot8.Schema{Ref: "#/components/requestBodies/CreateOrder"},
								},
							},
						},
					},
				},
			},
		},
		Components: &annot8.Components{
			Schemas:       map[string]annot8.Schema{},
			RequestBodies: map[string]annot8.RequestBody{},
		},
	}

	violations := annot8.ValidateRefs(spec)
	if len(violations) != 1 {
		t.Fatalf("expected one missing request body ref violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "#/components/requestBodies/CreateOrder") {
		t.Fatalf("unexpected violation: %q", violations[0])
	}
}
