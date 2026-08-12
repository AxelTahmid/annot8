package annot8fixtures_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AxelTahmid/annot8"
)

// @Summary Stream events
// @Tags test
// @Produce text/event-stream
// @Success 200 {object} map[string]string "SSE stream"
func sseProduceHandler(w http.ResponseWriter, r *http.Request) {}

// @Summary Plain JSON endpoint
// @Tags test
// @Success 200 {object} map[string]string "ok"
func jsonDefaultHandler(w http.ResponseWriter, r *http.Request) {}

// TestGenerateSpec_ProduceOverridesSuccessMediaType locks in @Produce support:
// an endpoint annotated with @Produce text/event-stream must declare that media
// type on its success response (SSE-aware client generators key off it), while
// endpoints without @Produce keep the application/json default.
func TestGenerateSpec_ProduceOverridesSuccessMediaType(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/v1/events", sseProduceHandler)
	r.Get("/api/v1/plain", jsonDefaultHandler)

	spec := annot8.NewGenerator().GenerateSpec(r, annot8.Config{
		Title:   "Produce Media Type Test",
		Version: "1.0.0",
	})

	sse := spec.Paths["/api/v1/events"].Get
	if sse == nil {
		t.Fatal("expected GET operation for /api/v1/events")
	}
	sseResp, ok := sse.Responses["200"]
	if !ok {
		t.Fatal("expected 200 response for SSE endpoint")
	}
	if _, ok = sseResp.Content["text/event-stream"]; !ok {
		t.Fatalf("expected text/event-stream content, got media types %v", mediaTypes(sseResp.Content))
	}
	if _, ok = sseResp.Content["application/json"]; ok {
		t.Fatal("SSE endpoint must not also declare application/json for the success response")
	}

	plain := spec.Paths["/api/v1/plain"].Get
	if plain == nil {
		t.Fatal("expected GET operation for /api/v1/plain")
	}
	plainResp, ok := plain.Responses["200"]
	if !ok {
		t.Fatal("expected 200 response for plain endpoint")
	}
	if _, ok = plainResp.Content["application/json"]; !ok {
		t.Fatalf("expected application/json default, got media types %v", mediaTypes(plainResp.Content))
	}
}

func mediaTypes(content map[string]annot8.MediaTypeObject) []string {
	out := make([]string, 0, len(content))
	for k := range content {
		out = append(out, k)
	}
	return out
}
