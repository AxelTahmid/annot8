package annot8_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AxelTahmid/annot8"
)

// --- Test Handlers for annotation parsing ---
// Handler with all annotation types
// @Summary Test summary
// @Description Test description
// @Tags foo,bar
// @Accept application/xml
// @Produce application/json
// @Security ApiKeyAuth
// @Param id path int true "ID param"
// @Param q query string false "Query param"
// @Success 200 {object} TestResponse "Success desc"
// @Failure 400 {object} ProblemDetails "Bad request"
func HandlerWithAnnotations() {}

func TestParseAnnotations_AllAnnotations(t *testing.T) {
	annotation, err := annot8.ParseAnnotations("annotations_test.go", "HandlerWithAnnotations")
	if err != nil {
		t.Fatalf("ParseAnnotations error: %v", err)
	}
	if annotation == nil {
		t.Fatal("ParseAnnotations returned nil")
	}
	if annotation.Summary != "Test summary" {
		t.Errorf("expected summary, got %q", annotation.Summary)
	}
	if annotation.Description != "Test description" {
		t.Errorf("expected description, got %q", annotation.Description)
	}
	if len(annotation.Tags) != 2 || annotation.Tags[0] != "foo" || annotation.Tags[1] != "bar" {
		t.Errorf("expected tags [foo bar], got %+v", annotation.Tags)
	}
	if len(annotation.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %+v", annotation.Parameters)
	}
	// Accept, Produce and Security
	if len(annotation.Accept) != 1 || annotation.Accept[0] != "application/xml" {
		t.Errorf("expected Accept [application/xml], got %v", annotation.Accept)
	}
	if len(annotation.Produce) != 1 || annotation.Produce[0] != "application/json" {
		t.Errorf("expected Produce [application/json], got %v", annotation.Produce)
	}
	if len(annotation.Security) != 1 || annotation.Security[0] != "ApiKeyAuth" {
		t.Errorf("expected Security [ApiKeyAuth], got %v", annotation.Security)
	}
	if annotation.Success == nil || annotation.Success.DataType != "TestResponse" {
		t.Errorf("expected success DataType 'TestResponse', got %+v", annotation.Success)
	}
	if len(annotation.Failures) != 1 || annotation.Failures[0].StatusCode != 400 {
		t.Errorf("expected failure 400, got %+v", annotation.Failures)
	}
}

func TestParseAnnotations_Empty(t *testing.T) {
	annotation, err := annot8.ParseAnnotations("annotations_test.go", "NonExistentHandler")
	if err != nil {
		t.Fatalf("ParseAnnotations error: %v", err)
	}
	if annotation != nil {
		t.Error("expected nil for non-existent handler")
	}
}

// Test handlers that reproduce the menu/coupon collision issue
// MenuList simulates the menu handler List function
// @Summary Get full menu
// @Description Retrieve the complete menu with all items
// @Tags menu
func MenuList() {}

// CouponList simulates the coupon handler List function
// @Summary List all coupons
// @Description Retrieve a list of all available coupons
// @Tags coupon
func CouponList() {}

// TestParseAnnotations_MenuCouponDistinct tests that menu and coupon handlers
// with the same function name "List" get their correct annotations
func TestParseAnnotations_MenuCouponDistinct(t *testing.T) {
	// Test menu handler annotations
	menuAnnotation, err := annot8.ParseAnnotations("annotations_test.go", "MenuList")
	if err != nil {
		t.Fatalf("ParseAnnotations for MenuList error: %v", err)
	}
	if menuAnnotation == nil {
		t.Fatal("ParseAnnotations for MenuList returned nil")
	}
	if menuAnnotation.Summary != "Get full menu" {
		t.Errorf("Menu handler: expected summary 'Get full menu', got %q", menuAnnotation.Summary)
	}

	// Test coupon handler annotations
	couponAnnotation, err := annot8.ParseAnnotations("annotations_test.go", "CouponList")
	if err != nil {
		t.Fatalf("ParseAnnotations for CouponList error: %v", err)
	}
	if couponAnnotation == nil {
		t.Fatal("ParseAnnotations for CouponList returned nil")
	}
	if couponAnnotation.Summary != "List all coupons" {
		t.Errorf("Coupon handler: expected summary 'List all coupons', got %q", couponAnnotation.Summary)
	}

	// Verify they are different
	if menuAnnotation.Summary == couponAnnotation.Summary {
		t.Errorf("Menu and coupon handlers should have different summaries, both got: %q", menuAnnotation.Summary)
	}

	// Test that calling them multiple times gives consistent results
	menuAnnotation2, err := annot8.ParseAnnotations("annotations_test.go", "MenuList")
	if err != nil {
		t.Fatalf("Second ParseAnnotations for MenuList error: %v", err)
	}
	if menuAnnotation2.Summary != "Get full menu" {
		t.Errorf("Second call: Menu handler summary changed to %q", menuAnnotation2.Summary)
	}

	couponAnnotation2, err := annot8.ParseAnnotations("annotations_test.go", "CouponList")
	if err != nil {
		t.Fatalf("Second ParseAnnotations for CouponList error: %v", err)
	}
	if couponAnnotation2.Summary != "List all coupons" {
		t.Errorf("Second call: Coupon handler summary changed to %q", couponAnnotation2.Summary)
	}
}

func TestParseAnnotations_WindowsStylePath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "handler.go")
	source := `package temp

// Handler demonstrates parsing with Windows-style paths
// @Summary Temp summary
// @Description This handler is used in tests only
func Handler() {}
`
	if err := os.WriteFile(filePath, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write temp handler: %v", err)
	}
	windowsPath := strings.ReplaceAll(filePath, string(filepath.Separator), "\\")

	annotation, err := annot8.ParseAnnotations(windowsPath, "Handler")
	if err != nil {
		t.Fatalf("ParseAnnotations with Windows path error: %v", err)
	}
	if annotation == nil {
		t.Fatal("ParseAnnotations returned nil for Windows-style path")
	}
	if annotation.Summary != "Temp summary" {
		t.Errorf("expected summary 'Temp summary', got %q", annotation.Summary)
	}
}
