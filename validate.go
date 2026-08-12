package annot8

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ValidationError contains one or more OpenAPI generation validation violations.
type ValidationError struct {
	Violations []string
}

func (e *ValidationError) Error() string {
	if len(e.Violations) == 0 {
		return "openapi validation failed"
	}
	return "openapi validation failed:\n- " + strings.Join(e.Violations, "\n- ")
}

// ValidateAnnotations reports operations missing required annotations.
func ValidateAnnotations(spec *Spec) []string {
	if spec == nil {
		return []string{"spec is nil"}
	}

	var violations []string
	for _, item := range collectOperations(spec) {
		op := item.op
		label := operationLabel(item.path, item.method, op)

		for _, parseErr := range op.annotationParseErrors {
			trimmed := strings.TrimSpace(parseErr)
			if trimmed == "" {
				continue
			}
			violations = append(violations, fmt.Sprintf("%s: annotation parse error: %s", label, trimmed))
		}

		if !op.hasSummaryAnnotation {
			violations = append(violations, fmt.Sprintf("%s: missing @Summary", label))
		}
		if !op.hasTagsAnnotation {
			violations = append(violations, fmt.Sprintf("%s: missing @Tags", label))
		}
		if !op.hasSuccessAnnotation {
			violations = append(violations, fmt.Sprintf("%s: missing @Success", label))
		}
	}

	sort.Strings(violations)
	return violations
}

// ValidateOperationIDs reports missing or duplicate operation IDs.
func ValidateOperationIDs(spec *Spec) []string {
	if spec == nil {
		return []string{"spec is nil"}
	}

	var violations []string
	labelsByID := make(map[string][]string)

	for _, item := range collectOperations(spec) {
		op := item.op
		label := operationLabel(item.path, item.method, op)

		id := strings.TrimSpace(op.OperationID)
		if id == "" {
			violations = append(violations, fmt.Sprintf("%s: missing operationId", label))
			continue
		}

		labelsByID[id] = append(labelsByID[id], label)
	}

	ids := make([]string, 0, len(labelsByID))
	for id := range labelsByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		labels := labelsByID[id]
		if len(labels) <= 1 {
			continue
		}
		sort.Strings(labels)
		violations = append(violations, fmt.Sprintf("duplicate operationId %q used by: %s", id, strings.Join(labels, ", ")))
	}

	sort.Strings(violations)
	return violations
}

// ValidateAmbiguousPaths reports path templates that can match the same URL.
func ValidateAmbiguousPaths(spec *Spec) []string {
	if spec == nil {
		return []string{"spec is nil"}
	}

	paths := make([]string, 0, len(spec.Paths))
	methodsByPath := make(map[string]map[string]struct{}, len(spec.Paths))
	for path := range spec.Paths {
		paths = append(paths, path)
		methodsByPath[path] = operationMethodsForPath(spec.Paths[path])
	}
	sort.Strings(paths)

	var violations []string
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			leftPath := paths[i]
			rightPath := paths[j]
			if !pathsAreAmbiguous(leftPath, rightPath) {
				continue
			}
			if !hasMethodOverlap(methodsByPath[leftPath], methodsByPath[rightPath]) {
				continue
			}
			violations = append(violations, fmt.Sprintf("ambiguous paths: %q and %q", leftPath, rightPath))
		}
	}

	sort.Strings(violations)
	return violations
}

// ValidateRefs reports unresolved #/components/* references.
func ValidateRefs(spec *Spec) []string {
	if spec == nil {
		return []string{"spec is nil"}
	}

	componentMembers := collectComponentMembers(spec)

	raw, err := json.Marshal(spec)
	if err != nil {
		return []string{fmt.Sprintf("marshal spec failed: %v", err)}
	}

	var doc any
	if err = json.Unmarshal(raw, &doc); err != nil {
		return []string{fmt.Sprintf("unmarshal spec failed: %v", err)}
	}

	refs := map[string]struct{}{}
	collectComponentRefs(doc, refs)

	var violations []string
	for ref := range refs {
		componentType, name, ok := parseComponentRef(ref)
		if !ok {
			continue
		}

		members, knownType := componentMembers[componentType]
		if !knownType {
			violations = append(violations, fmt.Sprintf("unknown component type in $ref %q", ref))
			continue
		}

		if _, ok = members[name]; ok {
			continue
		}
		violations = append(violations, fmt.Sprintf("missing component for $ref %q", ref))
	}

	sort.Strings(violations)
	return violations
}

func collectComponentMembers(spec *Spec) map[string]map[string]struct{} {
	components := map[string]map[string]struct{}{
		"schemas":         make(map[string]struct{}),
		"responses":       make(map[string]struct{}),
		"parameters":      make(map[string]struct{}),
		"examples":        make(map[string]struct{}),
		"requestBodies":   make(map[string]struct{}),
		"headers":         make(map[string]struct{}),
		"securitySchemes": make(map[string]struct{}),
		"links":           make(map[string]struct{}),
		"callbacks":       make(map[string]struct{}),
		"pathItems":       make(map[string]struct{}),
	}

	if spec == nil || spec.Components == nil {
		return components
	}

	for key := range spec.Components.Schemas {
		components["schemas"][key] = struct{}{}
	}
	for key := range spec.Components.Responses {
		components["responses"][key] = struct{}{}
	}
	for key := range spec.Components.Parameters {
		components["parameters"][key] = struct{}{}
	}
	for key := range spec.Components.Examples {
		components["examples"][key] = struct{}{}
	}
	for key := range spec.Components.RequestBodies {
		components["requestBodies"][key] = struct{}{}
	}
	for key := range spec.Components.Headers {
		components["headers"][key] = struct{}{}
	}
	for key := range spec.Components.SecuritySchemes {
		components["securitySchemes"][key] = struct{}{}
	}
	for key := range spec.Components.Links {
		components["links"][key] = struct{}{}
	}
	for key := range spec.Components.Callbacks {
		components["callbacks"][key] = struct{}{}
	}
	for key := range spec.Components.PathItems {
		components["pathItems"][key] = struct{}{}
	}

	return components
}

type operationEntry struct {
	path   string
	method string
	op     *Operation
}

func collectOperations(spec *Spec) []operationEntry {
	var paths []string
	for path := range spec.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	methods := []struct {
		name string
		get  func(PathItem) *Operation
	}{
		{name: "DELETE", get: func(p PathItem) *Operation { return p.Delete }},
		{name: "GET", get: func(p PathItem) *Operation { return p.Get }},
		{name: "HEAD", get: func(p PathItem) *Operation { return p.Head }},
		{name: "OPTIONS", get: func(p PathItem) *Operation { return p.Options }},
		{name: "PATCH", get: func(p PathItem) *Operation { return p.Patch }},
		{name: "POST", get: func(p PathItem) *Operation { return p.Post }},
		{name: "PUT", get: func(p PathItem) *Operation { return p.Put }},
		{name: "TRACE", get: func(p PathItem) *Operation { return p.Trace }},
	}

	entries := make([]operationEntry, 0, len(paths))
	for _, path := range paths {
		pathItem := spec.Paths[path]
		for _, method := range methods {
			if op := method.get(pathItem); op != nil {
				entries = append(entries, operationEntry{
					path:   path,
					method: method.name,
					op:     op,
				})
			}
		}
	}

	return entries
}

func operationLabel(path, method string, op *Operation) string {
	if op != nil && op.httpMethod != "" && op.routePattern != "" {
		return op.httpMethod + " " + op.routePattern
	}
	return method + " " + path
}

func collectComponentRefs(node any, refs map[string]struct{}) {
	switch n := node.(type) {
	case map[string]any:
		if rawRef, ok := n["$ref"]; ok {
			if ref, ok := rawRef.(string); ok && strings.HasPrefix(ref, "#/components/") {
				refs[ref] = struct{}{}
			}
		}
		for _, value := range n {
			collectComponentRefs(value, refs)
		}
	case []any:
		for _, value := range n {
			collectComponentRefs(value, refs)
		}
	}
}

func parseComponentRef(ref string) (componentType string, name string, ok bool) {
	if !strings.HasPrefix(ref, "#/components/") {
		return "", "", false
	}

	remaining := strings.TrimPrefix(ref, "#/components/")
	parts := strings.SplitN(remaining, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	componentType = strings.TrimSpace(parts[0])
	name = strings.TrimSpace(parts[1])
	if componentType == "" || name == "" {
		return "", "", false
	}

	return componentType, name, true
}

func pathsAreAmbiguous(left, right string) bool {
	leftSegments := splitPathSegments(left)
	rightSegments := splitPathSegments(right)

	if len(leftSegments) != len(rightSegments) {
		return false
	}

	hasParameterizedDifference := false
	for i := range leftSegments {
		a := leftSegments[i]
		b := rightSegments[i]

		if a == b {
			continue
		}

		aParam := isTemplatedSegment(a)
		bParam := isTemplatedSegment(b)

		if !aParam || !bParam {
			return false
		}

		hasParameterizedDifference = true
	}

	return hasParameterizedDifference
}

func operationMethodsForPath(pathItem PathItem) map[string]struct{} {
	methods := make(map[string]struct{}, 8)
	if pathItem.Get != nil {
		methods["GET"] = struct{}{}
	}
	if pathItem.Post != nil {
		methods["POST"] = struct{}{}
	}
	if pathItem.Put != nil {
		methods["PUT"] = struct{}{}
	}
	if pathItem.Patch != nil {
		methods["PATCH"] = struct{}{}
	}
	if pathItem.Delete != nil {
		methods["DELETE"] = struct{}{}
	}
	if pathItem.Options != nil {
		methods["OPTIONS"] = struct{}{}
	}
	if pathItem.Head != nil {
		methods["HEAD"] = struct{}{}
	}
	if pathItem.Trace != nil {
		methods["TRACE"] = struct{}{}
	}
	return methods
}

func hasMethodOverlap(left, right map[string]struct{}) bool {
	for method := range left {
		if _, ok := right[method]; ok {
			return true
		}
	}
	return false
}

func splitPathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func isTemplatedSegment(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}
