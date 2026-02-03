package annot8_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AxelTahmid/annot8"
)

// Mock handlers that simulate the actual menu and coupon handlers
type menuHandler struct{}

func (h *menuHandler) List(w http.ResponseWriter, r *http.Request) {}

type couponHandler struct{}

func (h *couponHandler) List(w http.ResponseWriter, r *http.Request) {}

// TestMenuCouponRouteMapping tests the specific scenario where menu and coupon
// handlers both have List methods and are mounted at different routes.
// This reproduces the real-world scenario from our application.
func TestMenuCouponRouteMapping(t *testing.T) {
	// Create handlers
	menuH := &menuHandler{}
	couponH := &couponHandler{}

	// Set up router similar to the real application
	r := chi.NewRouter()

	// Menu routes - similar to menu.Routes() in the real app
	r.Route("/api/v1/menu", func(r chi.Router) {
		r.Get("/", menuH.List)
	})

	// Coupon routes - similar to coupon.Routes() in the real app
	r.Route("/api/v1/coupon", func(r chi.Router) {
		r.Get("/", couponH.List)
	})

	// Discover routes
	routes, err := annot8.DiscoverRoutes(r)
	if err != nil {
		t.Fatalf("DiscoverRoutes error: %v", err)
	}

	// Log all discovered routes for debugging
	for i, route := range routes {
		t.Logf("Route %d: Method=%s, Pattern=%s, HandlerName=%s",
			i, route.Method, route.Pattern, route.HandlerName)
	}

	// Find our specific routes
	var menuRoute, couponRoute *annot8.RouteInfo
	for i := range routes {
		switch routes[i].Pattern {
		case "/api/v1/menu/":
			menuRoute = &routes[i]
		case "/api/v1/coupon/":
			couponRoute = &routes[i]
		}
	}

	if menuRoute == nil {
		t.Fatal("Menu route not found")
	}
	if couponRoute == nil {
		t.Fatal("Coupon route not found")
	}

	t.Logf("Menu route handler: %s", menuRoute.HandlerName)
	t.Logf("Coupon route handler: %s", couponRoute.HandlerName)

	// The critical test: handlers should be different
	if menuRoute.HandlerName == couponRoute.HandlerName {
		t.Errorf("Menu and coupon routes should map to different handlers, both got: %s",
			menuRoute.HandlerName)
	}

	// Test with the generator to see if annotations get cross-contaminated
	cfg := annot8.Config{Title: "Test API", Version: "1.0.0"}
	g := annot8.NewGenerator()
	spec := g.GenerateSpec(r, cfg)

	// Check paths exist
	if _, ok := spec.Paths["/api/v1/menu/"]; !ok {
		t.Error("Menu path not found in generated spec")
	}
	if _, ok := spec.Paths["/api/v1/coupon/"]; !ok {
		t.Error("Coupon path not found in generated spec")
	}

	// Check operations
	menuOps := spec.Paths["/api/v1/menu/"]
	couponOps := spec.Paths["/api/v1/coupon/"]

	menuGet := menuOps.Get
	couponGet := couponOps.Get

	if menuGet == nil {
		t.Error("Menu GET operation not found")
	}
	if couponGet == nil {
		t.Error("Coupon GET operation not found")
	}

	t.Logf("Menu operation ID: %s, Summary: %s", menuGet.OperationID, menuGet.Summary)
	t.Logf("Coupon operation ID: %s, Summary: %s", couponGet.OperationID, couponGet.Summary)

	// Verify operations are distinct
	if menuGet.OperationID == couponGet.OperationID {
		t.Errorf("Menu and coupon operations should have different IDs, both got: %s",
			menuGet.OperationID)
	}
}
