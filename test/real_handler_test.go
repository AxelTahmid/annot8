package annot8_test

import (
	"path/filepath"
	"testing"

	annot8 "github.com/AxelTahmid/annot8"
)

// Test_RealHandlerAnnotations tests annotation parsing against the real handlers.
func Test_RealHandlerAnnotations(t *testing.T) {
	menuPath, err := filepath.Abs(filepath.Join("..", "..", "..", "internal", "api", "menu", "handler.go"))
	if err != nil {
		t.Fatalf("failed to resolve menu handler path: %v", err)
	}
	couponPath, err := filepath.Abs(filepath.Join("..", "..", "..", "internal", "api", "coupon", "handler.go"))
	if err != nil {
		t.Fatalf("failed to resolve coupon handler path: %v", err)
	}

	menuAnnotation, err := annot8.ParseAnnotations(menuPath, "FullMenu")
	if err != nil {
		t.Fatalf("ParseAnnotations for menu handler error: %v", err)
	}
	couponAnnotation, err := annot8.ParseAnnotations(couponPath, "List")
	if err != nil {
		t.Fatalf("ParseAnnotations for coupon handler error: %v", err)
	}

	menuSummary := ""
	if menuAnnotation != nil {
		menuSummary = menuAnnotation.Summary
	}
	couponSummary := ""
	if couponAnnotation != nil {
		couponSummary = couponAnnotation.Summary
	}

	t.Logf("Menu handler summary: %q", menuSummary)
	t.Logf("Coupon handler summary: %q", couponSummary)

	if menuSummary != "" && couponSummary != "" && menuSummary == couponSummary {
		t.Errorf("Menu and coupon handlers should differ, both got: %q", menuSummary)
	}

	if menuAnnotation != nil && menuAnnotation.Summary != "Get menu" {
		t.Errorf("Menu handler: expected 'Get menu', got %q", menuAnnotation.Summary)
	}
	if couponAnnotation != nil && couponAnnotation.Summary != "List coupons" {
		t.Errorf("Coupon handler: expected 'List coupons', got %q", couponAnnotation.Summary)
	}

	menuAnnotation2, err := annot8.ParseAnnotations(menuPath, "FullMenu")
	if err != nil {
		t.Fatalf("Second ParseAnnotations for menu handler error: %v", err)
	}
	if menuAnnotation2 != nil && menuAnnotation2.Summary != menuSummary {
		t.Errorf("Menu handler summary changed from %q to %q on second call", menuSummary, menuAnnotation2.Summary)
	}
}
