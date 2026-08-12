package annot8

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/go-chi/chi/v5"
)

type GenerateParams struct {
	Router         chi.Router
	Config         Config
	FilePath       string
	RenameFunction ModelNameFunc
	Validate       bool
}

// GenerateOpenAPISpecFile generates the OpenAPI spec and writes it to the given file path.
func GenerateOpenAPISpecFile(p *GenerateParams) error {
	slog.Debug("[annot8] GenerateOpenAPISpecFile: generating OpenAPI spec", "filePath", p.FilePath)

	ensureTypeIndex()

	renameFunc := p.RenameFunction
	if renameFunc == nil {
		renameFunc = DefaultModelNameFunc
	}

	gen := NewGeneratorWithCache(typeIndex)
	gen.SetModelNameFunc(renameFunc)

	spec := gen.GenerateSpec(p.Router, p.Config)

	if p.Validate {
		var violations []string
		violations = append(violations, ValidateAnnotations(&spec)...)
		violations = append(violations, ValidateOperationIDs(&spec)...)
		violations = append(violations, ValidateAmbiguousPaths(&spec)...)
		violations = append(violations, ValidateRefs(&spec)...)
		if len(violations) > 0 {
			return &ValidationError{Violations: violations}
		}
	}

	slog.Debug("[annot8] GenerateOpenAPISpecFile: writing OpenAPI spec to file", "version", spec.Info.Version)

	// Write to a temp file and rename: the target is embedded into the server
	// binary, so a failed encode must never leave a truncated spec behind.
	tmp, err := os.CreateTemp(filepath.Dir(p.FilePath), ".openapi-*.json")
	if err != nil {
		slog.Error("[annot8] GenerateOpenAPISpecFile: failed to create temp file", "err", err, "path", p.FilePath)
		return err
	}
	// Cleans up the temp file on every failure path; after a successful rename
	// the name no longer exists, so the removal is expected to fail.
	defer func() {
		if removeErr := os.Remove(tmp.Name()); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			slog.Warn("[annot8] GenerateOpenAPISpecFile: failed to remove temp file",
				"err", removeErr, "path", tmp.Name())
		}
	}()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err = enc.Encode(spec); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			slog.Warn("[annot8] GenerateOpenAPISpecFile: failed to close temp file", "err", closeErr)
		}
		slog.Error("[annot8] GenerateOpenAPISpecFile: failed to write file", "err", err)
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), p.FilePath); err != nil {
		slog.Error("[annot8] GenerateOpenAPISpecFile: failed to move spec into place", "err", err)
		return err
	}

	slog.Debug("[annot8] GenerateOpenAPISpecFile: annot8.json written successfully")
	return nil
}

// SwaggerUIHandler returns an HTTP handler that renders the Scalar API reference UI.
// Pass raw JSON spec bytes as the second argument to embed the spec inline.
// This is required for relative or auth-protected spec URLs, because the Scalar
// library only resolves http(s) URLs — anything else is treated as a local file path.
func SwaggerUIHandler(specURL string, specContent ...[]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		opts := scalar.Options{
			SpecURL: specURL,
			CustomOptions: scalar.CustomOptions{
				PageTitle: "API Documentation",
			},
			DarkMode:           true,
			ShowSidebar:        true,
			HideModels:         false,
			HideDownloadButton: false,
			Layout:             scalar.LayoutModern,
		}
		if len(specContent) > 0 && len(specContent[0]) > 0 {
			opts.SpecURL = ""
			opts.SpecContent = string(specContent[0])
		}
		htmlContent, err := scalar.ApiReferenceHTML(&opts)

		if err != nil {
			slog.Error("[annot8] SwaggerUIHandler: failed to generate API reference HTML", "error", err)
			http.Error(w, "Failed to generate API reference", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err = fmt.Fprintln(w, htmlContent); err != nil {
			// Status is already committed, so there is no error response left
			// to send — the client simply went away mid-page.
			slog.Debug("[annot8] SwaggerUIHandler: failed to write API reference", "error", err)
		}
	}
}
