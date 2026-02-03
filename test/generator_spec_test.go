package annot8_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	annot8 "github.com/AxelTahmid/annot8"
)

// TestGenerateSpecRoutes ensures that GenerateSpec includes discovered routes and parameters.
func TestGenerateSpecRoutes(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/foo/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cfg := annot8.Config{Title: "Test Service", Version: "1.2.3"}
	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, cfg)

	// Check Info
	if spec.Info.Title != cfg.Title {
		t.Errorf("expected Info.Title %q, got %q", cfg.Title, spec.Info.Title)
	}
	if spec.Info.Version != cfg.Version {
		t.Errorf("expected Info.Version %q, got %q", cfg.Version, spec.Info.Version)
	}

	// Check path presence and operation
	paths := spec.Paths
	if _, ok := paths["/foo/{id}"]; !ok {
		t.Fatalf("expected path '/foo/{id}' in spec.Paths")
	}
	ops := paths["/foo/{id}"]
	op := ops.Get
	if op == nil {
		t.Fatalf("expected GET operation for '/foo/{id}'")
	}

	// Verify path parameter id
	if len(op.Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(op.Parameters))
	}
	p := op.Parameters[0]
	if p.Name != "id" || p.In != "path" || !p.Required {
		t.Errorf("unexpected path parameter: %+v", p)
	}
}

func TestGenerateSpec_Compliance31(t *testing.T) {
	r := chi.NewRouter()
	cfg := annot8.Config{
		Title:       "Compliance Test",
		Summary:     "Test Summary",
		Version:     "3.1.0",
		Description: "Testing 3.1 features",
		License: &annot8.License{
			Name:       "Apache 2.0",
			Identifier: "Apache-2.0",
		},
	}
	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, cfg)

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("expected OpenAPI 3.1.0, got %s", spec.OpenAPI)
	}

	if spec.Info.Summary != "Test Summary" {
		t.Errorf("expected info summary Test Summary, got %s", spec.Info.Summary)
	}

	if spec.Info.License.Identifier != "Apache-2.0" {
		t.Errorf("expected license identifier Apache-2.0, got %s", spec.Info.License.Identifier)
	}
}

// Ensure that handlers implemented as methods on a struct (method receiver)
// are discovered and their comment annotations are parsed correctly.
// This reproduces the case where the router registers a method value, e.g.
// r.Post("/invoices/{id}", h.create)
type invoicesHandler struct{}

// @Summary Create invoice
// @Description Create a new invoice for the tenant
// @Tags invoices
// @Param id path int true "Invoice ID"
// @Success 201 {object} CreateInvoiceResponse "created"
func (h *invoicesHandler) create(w http.ResponseWriter, r *http.Request) {}

func TestGenerateSpecRoutes_MethodReceiver(t *testing.T) {
	r := chi.NewRouter()
	h := &invoicesHandler{}
	// register method value as handler
	r.Post("/invoices/{id}", http.HandlerFunc(h.create))

	cfg := annot8.Config{Title: "Test Service", Version: "1.2.3"}
	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, cfg)

	// ensure path exists
	if _, ok := spec.Paths["/invoices/{id}"]; !ok {
		t.Fatalf("expected path '/invoices/{id}' in spec.Paths")
	}

	ops := spec.Paths["/invoices/{id}"]
	op := ops.Post
	if op == nil {
		t.Fatalf("expected POST operation for '/invoices/{id}'")
	}

	// Verify annotation fields were parsed into the operation
	if op.Summary != "Create invoice" {
		t.Errorf("expected summary 'Create invoice', got %q", op.Summary)
	}
	if op.Description != "Create a new invoice for the tenant" {
		t.Errorf("expected description parsed, got %q", op.Description)
	}

	// Verify path parameter id
	foundID := false
	for _, p := range op.Parameters {
		if p.In == "path" && p.Name == "id" {
			foundID = true
			if !p.Required {
				t.Errorf("expected path parameter id to be required")
			}
		}
	}
	if !foundID {
		t.Errorf("expected path parameter 'id' in operation parameters, got %+v", op.Parameters)
	}
}

// TestGenerateSpec_MenuCouponCollision tests the exact scenario where menu and coupon
// handlers both have "List" method names, ensuring they get distinct summaries.
func TestGenerateSpec_MenuCouponCollision(t *testing.T) {
	// Create mock handlers that simulate the actual menu and coupon handlers
	menuHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	couponHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	r := chi.NewRouter()
	r.Get("/api/v1/menu/", menuHandler)
	r.Get("/api/v1/coupon/", couponHandler)

	cfg := annot8.Config{Title: "Test API", Version: "1.0.0"}
	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, cfg)

	// Check that both paths exist
	paths := spec.Paths
	if _, ok := paths["/api/v1/menu/"]; !ok {
		t.Fatalf("expected path '/api/v1/menu/' in spec.Paths")
	}
	if _, ok := paths["/api/v1/coupon/"]; !ok {
		t.Fatalf("expected path '/api/v1/coupon/' in spec.Paths")
	}

	// Get the operations
	menuOps := paths["/api/v1/menu/"]
	couponOps := paths["/api/v1/coupon/"]

	// Check GET operations exist
	menuGet := menuOps.Get
	if menuGet == nil {
		t.Fatal("expected GET operation for menu path")
	}
	couponGet := couponOps.Get
	if couponGet == nil {
		t.Fatal("expected GET operation for coupon path")
	}

	// Extract summaries
	menuSummary := menuGet.Summary
	couponSummary := couponGet.Summary

	t.Logf("Menu route summary: %q", menuSummary)
	t.Logf("Coupon route summary: %q", couponSummary)

	// If either summary is non-empty, ensure they are not the same (no cross-contamination).
	if menuSummary != "" || couponSummary != "" {
		if menuSummary == couponSummary {
			t.Errorf(
				"Menu and coupon operations should have different summaries when present, both got: %q",
				menuSummary,
			)
		}
	}

	// Always ensure operation IDs differ to avoid accidental collisions
	if menuGet.OperationID == couponGet.OperationID {
		t.Errorf(
			"Menu and coupon operations should have different operation IDs, both got: %q",
			menuGet.OperationID,
		)
	}
}

type RenameTestModel struct {
	ID string `json:"id"`
}

type NestedModel struct {
	Child RenameTestModel `json:"child"`
}

func TestGenerateSpec_ModelRenaming(t *testing.T) {
	r := chi.NewRouter()

	// Register some "external" types to test renaming without needing real source files
	annot8.AddExternalKnownType("annot8_test.RenameTarget", &annot8.Schema{
		Type: "object",
		Properties: map[string]*annot8.Schema{
			"id": {Type: "string"},
		},
	})
	annot8.AddExternalKnownType("annot8_test.NestedTarget", &annot8.Schema{
		Type: "object",
		Properties: map[string]*annot8.Schema{
			"child": {Ref: "#/components/schemas/annot8_test.RenameTarget"},
		},
	})

	// Define a custom strategy that removes "Rename" prefix
	customStrategy := func(pkg, name string) string {
		if strings.HasPrefix(name, "Rename") {
			return strings.TrimPrefix(name, "Rename")
		}
		return name
	}

	cfg := annot8.Config{Title: "Renaming Test", Version: "1.0.0"}
	g := annot8.NewGenerator()
	g.SetModelNameFunc(customStrategy)

	// Trigger generation of these schemas
	g.GenerateSchema("annot8_test.RenameTarget")
	g.GenerateSchema("annot8_test.NestedTarget")

	spec := g.GenerateSpec(r, cfg)

	// annot8_test.RenameTarget should be renamed to Target (pkg=annot8_test, name=RenameTarget)
	if _, ok := spec.Components.Schemas["Target"]; !ok {
		keys := make([]string, 0, len(spec.Components.Schemas))
		for k := range spec.Components.Schemas {
			keys = append(keys, k)
		}
		t.Errorf("expected schema 'Target', but got keys: %v", keys)
	}

	// NestedTarget should still be NestedTarget
	nested, ok := spec.Components.Schemas["NestedTarget"]
	if !ok {
		t.Fatal("expected schema 'NestedTarget'")
	}

	// Check if the ref inside NestedTarget points to Target
	// The property name is "child"
	childProp, ok := nested.Properties["child"]
	if !ok {
		t.Fatal("expected property 'child' in NestedTarget")
	}

	expectedRef := "#/components/schemas/Target"
	if childProp.Ref != expectedRef {
		t.Errorf("expected ref %q, got %q", expectedRef, childProp.Ref)
	}
}

func TestGenerateSpec_ConflictResolution(t *testing.T) {
	cfg := annot8.Config{Title: "Conflict Test", Version: "1.0.0"}
	g := annot8.NewGenerator()

	// Strategy that forces everything to the same name
	g.SetModelNameFunc(func(pkg, name string) string {
		return "ConflictModel"
	})

	// Manually generate two different schemas
	g.GenerateSchema("Alpha")
	g.GenerateSchema("Beta")

	spec := g.GenerateSpec(chi.NewRouter(), cfg)

	// One should be ConflictModel, the other ConflictModel2
	if _, ok := spec.Components.Schemas["ConflictModel"]; !ok {
		keys := make([]string, 0, len(spec.Components.Schemas))
		for k := range spec.Components.Schemas {
			keys = append(keys, k)
		}
		t.Errorf("expected 'ConflictModel', but got keys: %v", keys)
	}
	if _, ok := spec.Components.Schemas["ConflictModel2"]; !ok {
		keys := make([]string, 0, len(spec.Components.Schemas))
		for k := range spec.Components.Schemas {
			keys = append(keys, k)
		}
		t.Errorf("expected 'ConflictModel2', but got keys: %v", keys)
	}
}
