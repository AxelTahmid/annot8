package annot8

import (
	"errors"
	"go/ast"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// buildOperation turns a Chi route into an OpenAPI operation.
func (g *Generator) buildOperation(
	handler http.Handler,
	route, method string,
	middlewares []func(http.Handler) http.Handler,
	securityCfg SecurityInferenceConfig,
) Operation {
	slog.Debug("[annot8] buildOperation: called", "route", route, "method", method)

	handlerInfo := g.extractHandlerInfo(handler, route)

	var annotations *Annotation
	var annotationParseErrors []string
	if handlerInfo != nil && handlerInfo.File != "" {
		var err error
		annotations, err = ParseAnnotations(handlerInfo.File, handlerInfo.FunctionName)
		if err != nil {
			slog.Warn("[annot8] buildOperation: annotations parse error", "error", err)
			annotationParseErrors = extractAnnotationParseErrors(err)
		}
	}

	op := Operation{
		OperationID:           generateOperationID(method, route),
		Responses:             g.buildResponses(annotations),
		routePattern:          route,
		httpMethod:            strings.ToUpper(method),
		annotationParseErrors: annotationParseErrors,
	}

	// Merge path parameters derived from the route itself.
	op.Parameters = append(op.Parameters, extractPathParameters(route)...)

	// Apply annotation-derived metadata.
	if annotations != nil {
		op.hasSummaryAnnotation = strings.TrimSpace(annotations.Summary) != ""
		op.hasTagsAnnotation = len(annotations.Tags) > 0
		op.hasSuccessAnnotation = len(annotations.Successes) > 0

		op.Summary = annotations.Summary
		op.Description = annotations.Description
		op.Tags = append(op.Tags, annotations.Tags...)

		for _, param := range annotations.Parameters {
			if param.In == "body" {
				continue
			}

			if expanded := g.expandQueryObjectParam(param); len(expanded) > 0 {
				for _, qp := range expanded {
					op.Parameters = upsertParameter(op.Parameters, qp)
				}
				continue
			}

			op.Parameters = upsertParameter(op.Parameters, Parameter{
				Name:        param.Name,
				In:          param.In,
				Description: param.Description,
				Required:    param.Required,
				Schema:      normalizeParameterSchema(param.In, g.schemaGen.GenerateSchema(param.Type)),
			})
		}

	}

	if len(op.Tags) == 0 {
		op.Tags = []string{extractResourceFromRoute(route)}
	}

	if requestBody := g.buildRequestBody(annotations); requestBody != nil {
		op.RequestBody = requestBody
	}

	if inferred := inferOperationSecurity(route, method, middlewares, securityCfg); len(inferred) > 0 {
		op.Security = inferred
	}

	// Explicit @Security annotations win over middleware inference — they are
	// the only way to document authorization enforced inside handler bodies,
	// which the middleware scan cannot see.
	if annotations != nil && len(annotations.Security) > 0 {
		op.Security = nil
		for _, scheme := range annotations.Security {
			op.Security = append(op.Security, SecurityRequirement{scheme: {}})
		}
	}

	if perms := g.resolveACLPermissions(route, method, handlerInfo, middlewares); len(perms) > 0 {
		if securityCfg.EnableACLBearerFromPermissions && len(op.Security) == 0 && aclPermissionsRequireBearer(perms) {
			op.Security = []SecurityRequirement{{"BearerAuth": {}}}
		}

		aclInfo := "\n\nAccess control:\n- " + strings.Join(perms, "\n- ")
		if op.Description != "" {
			op.Description += aclInfo
		} else {
			op.Description = "This endpoint requires authentication." + aclInfo
		}
	}

	sortOperationParameters(op.Parameters)

	slog.Debug("[annot8] buildOperation: completed", "operationId", op.OperationID)
	return op
}

func extractAnnotationParseErrors(err error) []string {
	if err == nil {
		return nil
	}

	var parsingErr *AnnotationParsingError
	if errors.As(err, &parsingErr) && len(parsingErr.Messages) > 0 {
		out := make([]string, 0, len(parsingErr.Messages))
		for _, msg := range parsingErr.Messages {
			trimmed := strings.TrimSpace(msg)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
		if len(out) > 0 {
			return out
		}
	}

	return []string{strings.TrimSpace(err.Error())}
}

func (g *Generator) expandQueryObjectParam(param ParamAnnotation) []Parameter {
	if param.In != "query" {
		return nil
	}
	if g.schemaGen == nil || g.schemaGen.typeIndex == nil {
		return nil
	}

	typeSpec := g.resolveTypeSpec(param.Type)
	if typeSpec == nil {
		return nil
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return nil
	}

	params := make([]Parameter, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		name := queryParamNameFromField(field)
		if name == "" {
			continue
		}

		for _, fieldName := range field.Names {
			if !ast.IsExported(fieldName.Name) {
				continue
			}
			params = append(params, Parameter{
				Name:        name,
				In:          "query",
				Description: fieldDescription(field),
				Required:    fieldRequired(field),
				Schema:      normalizeParameterSchema("query", g.schemaGen.convertFieldType(field.Type)),
			})
			break
		}
	}

	if len(params) == 0 {
		return nil
	}
	return params
}

func (g *Generator) resolveTypeSpec(typeName string) *ast.TypeSpec {
	qualified := g.schemaGen.getQualifiedTypeName(typeName)
	if ts := g.schemaGen.typeIndex.LookupQualifiedType(qualified); ts != nil {
		return ts
	}
	ts, _ := g.schemaGen.typeIndex.LookupUnqualifiedType(typeName)
	return ts
}

func queryParamNameFromField(field *ast.Field) string {
	tag := ""
	if field.Tag != nil {
		tag = strings.Trim(field.Tag.Value, "`")
	}

	if sourceTag := extractTag(tag, "source"); sourceTag != "" {
		parts := strings.SplitN(sourceTag, ":", 2)
		if len(parts) != 2 || parts[0] != "query" {
			return ""
		}
		return parts[1]
	}

	jsonName := extractJSONTag(tag)
	if jsonName == "-" {
		return ""
	}
	if jsonName != "" {
		return jsonName
	}

	if len(field.Names) == 0 {
		return ""
	}
	return field.Names[0].Name
}

func fieldDescription(field *ast.Field) string {
	if field.Doc != nil {
		return strings.TrimSpace(field.Doc.Text())
	}
	if field.Comment != nil {
		return strings.TrimSpace(field.Comment.Text())
	}
	return ""
}

func fieldRequired(field *ast.Field) bool {
	return !isPointerType(field.Type)
}

func normalizeParameterSchema(in string, schema *Schema) *Schema {
	if in != "query" || schema == nil {
		return schema
	}
	copied := cloneSchema(schema)
	removeNullability(copied)
	return copied
}

func cloneSchema(s *Schema) *Schema {
	if s == nil {
		return nil
	}
	out := *s
	if s.Properties != nil {
		out.Properties = make(map[string]*Schema, len(s.Properties))
		for k, v := range s.Properties {
			out.Properties[k] = cloneSchema(v)
		}
	}
	if s.Items != nil {
		out.Items = cloneSchema(s.Items)
	}
	if s.OneOf != nil {
		out.OneOf = make([]*Schema, 0, len(s.OneOf))
		for _, v := range s.OneOf {
			out.OneOf = append(out.OneOf, cloneSchema(v))
		}
	}
	if s.AnyOf != nil {
		out.AnyOf = make([]*Schema, 0, len(s.AnyOf))
		for _, v := range s.AnyOf {
			out.AnyOf = append(out.AnyOf, cloneSchema(v))
		}
	}
	if s.AllOf != nil {
		out.AllOf = make([]*Schema, 0, len(s.AllOf))
		for _, v := range s.AllOf {
			out.AllOf = append(out.AllOf, cloneSchema(v))
		}
	}
	if s.Not != nil {
		out.Not = cloneSchema(s.Not)
	}
	if ap, ok := s.AdditionalProperties.(*Schema); ok {
		out.AdditionalProperties = cloneSchema(ap)
	}
	return &out
}

func removeNullability(s *Schema) {
	if s == nil {
		return
	}

	switch t := s.Type.(type) {
	case []string:
		filtered := make([]string, 0, len(t))
		for _, v := range t {
			if v != "null" {
				filtered = append(filtered, v)
			}
		}
		switch len(filtered) {
		case 0:
			s.Type = nil
		case 1:
			s.Type = filtered[0]
		default:
			s.Type = filtered
		}
	case []any:
		filtered := make([]any, 0, len(t))
		for _, v := range t {
			if str, ok := v.(string); ok && str == "null" {
				continue
			}
			filtered = append(filtered, v)
		}
		switch len(filtered) {
		case 0:
			s.Type = nil
		case 1:
			s.Type = filtered[0]
		default:
			s.Type = filtered
		}
	}

	if len(s.AnyOf) > 0 {
		filtered := make([]*Schema, 0, len(s.AnyOf))
		for _, sub := range s.AnyOf {
			if sub != nil && sub.Type == "null" {
				continue
			}
			removeNullability(sub)
			filtered = append(filtered, sub)
		}
		if len(filtered) == 1 && s.Ref == "" && s.Type == nil && len(s.Properties) == 0 && s.Items == nil {
			*s = *filtered[0]
		} else {
			s.AnyOf = filtered
		}
	}

	if strings.HasPrefix(s.Description, "Nullable ") {
		s.Description = strings.TrimPrefix(s.Description, "Nullable ")
		if s.Description != "" {
			runes := []rune(s.Description)
			runes[0] = unicode.ToUpper(runes[0])
			s.Description = string(runes)
		}
	}
}

// successMediaType resolves the media type for success responses. The first
// @Produce annotation wins (e.g. "text/event-stream" for SSE endpoints);
// endpoints without one keep the JSON default.
func successMediaType(annotations *Annotation) string {
	if annotations != nil && len(annotations.Produce) > 0 && annotations.Produce[0] != "" {
		return annotations.Produce[0]
	}
	return "application/json"
}

// buildResponses assembles HTTP responses using annotations as hints.
func (g *Generator) buildResponses(annotations *Annotation) Responses {
	slog.Debug("[annot8] buildResponses: called")

	responses := make(Responses)

	if annotations != nil && len(annotations.Successes) > 0 {
		for _, success := range annotations.Successes {
			statusCode := strconv.Itoa(success.StatusCode)
			if _, exists := responses[statusCode]; exists {
				// First declaration wins when duplicate status codes are declared.
				continue
			}

			schema := g.generateResponseSchema(success.DataType)
			if success.IsWrapped {
				props := map[string]*Schema{
					"message": {Type: "string"},
					"data":    schema,
				}

				// Only include meta if the data type is a slice (implies pagination).
				if strings.HasPrefix(strings.TrimPrefix(success.DataType, "*"), "[]") {
					props["meta"] = &Schema{Ref: "#/components/schemas/PaginationMeta"}
				}

				schema = &Schema{
					Type:       "object",
					Required:   []string{"message"},
					Properties: props,
				}
			}

			responses[statusCode] = Response{
				Description: success.Description,
				Content: map[string]MediaTypeObject{
					successMediaType(annotations): {
						Schema: schema,
					},
				},
			}
		}
	} else {
		responses["200"] = Response{
			Description: "Successful response",
			Content: map[string]MediaTypeObject{
				"application/json": {Schema: &Schema{Type: "object"}},
			},
		}
	}

	if annotations != nil {
		for _, failure := range annotations.Failures {
			statusCode := strconv.Itoa(failure.StatusCode)
			// Use the type declared in the annotation when present; otherwise use
			// the portable RFC 9457-style ProblemDetails component.
			var failureSchema *Schema
			if failure.Type != "" {
				failureSchema = g.schemaGen.GenerateSchema(failure.Type)
			} else {
				failureSchema = &Schema{Ref: "#/components/schemas/ProblemDetails"}
			}
			responses[statusCode] = Response{
				Description: failure.Description,
				Content: map[string]MediaTypeObject{
					"application/problem+json": {
						Schema: failureSchema,
					},
				},
			}
		}
	}

	standardErrors := map[string]Response{
		"400": {Description: "Bad Request", Content: problemJSON()},
		"401": {Description: "Unauthorized", Content: problemJSON()},
		"403": {Description: "Forbidden", Content: problemJSON()},
		"404": {Description: "Not Found", Content: problemJSON()},
		"500": {Description: "Internal Server Error", Content: problemJSON()},
	}

	for code, response := range standardErrors {
		if _, exists := responses[code]; !exists {
			responses[code] = response
		}
	}

	slog.Debug("[annot8] buildResponses: completed", "response_count", len(responses))
	return responses
}

func problemJSON() map[string]MediaTypeObject {
	return map[string]MediaTypeObject{
		"application/problem+json": {
			Schema: &Schema{Ref: "#/components/schemas/ProblemDetails"},
		},
	}
}

// buildRequestBody constructs a request body definition.
func (g *Generator) buildRequestBody(annotations *Annotation) *RequestBody {
	slog.Debug("[annot8] buildRequestBody: called")

	var (
		schema      *Schema
		description = "Request body"
		required    bool
	)

	if annotations != nil {
		for _, param := range annotations.Parameters {
			if param.In != "body" {
				continue
			}
			slog.Debug("[annot8] buildRequestBody: found body parameter", "type", param.Type)

			schema = g.schemaGen.GenerateSchema(param.Type)
			if param.Description != "" {
				description = param.Description
			}
			required = param.Required
			break
		}
	}

	if schema == nil {
		return nil
	}

	return &RequestBody{
		Description: description,
		Required:    required,
		Content: map[string]MediaTypeObject{
			"application/json": {Schema: schema},
		},
	}
}

// generateResponseSchema resolves the schema referenced by an annotation.
func (g *Generator) generateResponseSchema(dataType string) *Schema {
	slog.Debug("[annot8] generateResponseSchema: called", "dataType", dataType)

	if dataType == "" {
		return &Schema{Type: "object"}
	}

	switch {
	case strings.HasPrefix(dataType, "[]"):
		itemType := strings.TrimPrefix(dataType, "[]")
		return &Schema{
			Type:  "array",
			Items: g.schemaGen.GenerateSchema(itemType),
		}
	case strings.HasPrefix(dataType, "*"):
		return g.schemaGen.GenerateSchema(strings.TrimPrefix(dataType, "*"))
	default:
		return g.schemaGen.GenerateSchema(dataType)
	}
}

// buildTags produces tag entries sorted for determinism.
func (g *Generator) buildTags(tagNames map[string]bool) []Tag {
	slog.Debug("[annot8] buildTags: called", "tag_count", len(tagNames))

	var tags []Tag
	for name := range tagNames {
		tags = append(tags, Tag{
			Name:        name,
			Description: capitalize(name) + " related operations",
		})
	}

	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags
}

// convertRouteToOpenAPIPath removes regex constraints from Chi-style parameters.
func convertRouteToOpenAPIPath(route string) string {
	parts := strings.Split(route, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			inner := strings.Trim(part, "{}")
			if before, _, ok := strings.Cut(inner, ":"); ok {
				parts[i] = "{" + before + "}"
			}
		}
	}
	path := strings.Join(parts, "/")
	if path == "" {
		return "/"
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if path == "" {
		return "/"
	}
	return path
}

// extractPathParameters converts route parameters into OpenAPI parameters.
func extractPathParameters(route string) []Parameter {
	var params []Parameter

	for _, part := range strings.Split(route, "/") {
		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}

		paramName := strings.Trim(part, "{}")
		if colon := strings.Index(paramName, ":"); colon != -1 {
			paramName = paramName[:colon]
		}

		params = append(params, Parameter{
			Name:     paramName,
			In:       "path",
			Required: true,
			Schema:   &Schema{Type: "string"},
		})
	}

	return params
}

// generateOperationID creates a stable operation ID based on method and route.
func generateOperationID(method, route string) string {
	normalizedRoute := convertRouteToOpenAPIPath(route)
	segments := strings.Split(strings.Trim(normalizedRoute, "/"), "/")

	parts := make([]string, 0, len(segments)+1)
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			if colon := strings.Index(name, ":"); colon != -1 {
				name = name[:colon]
			}
			token := toPascalIdentifier(name)
			if token == "" {
				token = "Param"
			}
			parts = append(parts, "By"+token)
			continue
		}

		token := toPascalIdentifier(segment)
		if token != "" {
			parts = append(parts, token)
		}
	}

	if len(parts) == 0 {
		parts = append(parts, "Root")
	}

	return strings.ToLower(method) + strings.Join(parts, "")
}

func toPascalIdentifier(segment string) string {
	words := splitIdentifierWords(segment)
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	for _, word := range words {
		lower := strings.ToLower(word)
		r, size := utf8.DecodeRuneInString(lower)
		if size == 0 {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
		b.WriteString(lower[size:])
	}

	token := b.String()
	if token == "" {
		return ""
	}

	r, _ := utf8.DecodeRuneInString(token)
	if unicode.IsDigit(r) {
		return "N" + token
	}

	return token
}

func splitIdentifierWords(s string) []string {
	var (
		words   []string
		current []rune
	)

	flush := func() {
		if len(current) == 0 {
			return
		}
		words = append(words, string(current))
		current = current[:0]
	}

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()

	return words
}

// extractResourceFromRoute returns the first meaningful route segment.
func extractResourceFromRoute(route string) string {
	for _, part := range strings.Split(strings.Trim(route, "/"), "/") {
		if part != "" && part != "api" && part != "v1" && !strings.Contains(part, "{") {
			return part
		}
	}
	return "default"
}

// capitalize upper-cases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// upsertParameter merges a parameter into an existing slice.
func upsertParameter(params []Parameter, p Parameter) []Parameter {
	for i, existing := range params {
		if existing.Name != p.Name || existing.In != p.In {
			continue
		}

		if p.Description != "" {
			existing.Description = p.Description
		}
		if p.Required {
			existing.Required = true
		}
		if p.Schema != nil {
			existing.Schema = p.Schema
		}
		params[i] = existing
		return params
	}

	return append(params, p)
}

func sortOperationParameters(params []Parameter) {
	if len(params) <= 1 {
		return
	}

	sort.Slice(params, func(i, j int) bool {
		left := params[i]
		right := params[j]

		leftRank := parameterLocationRank(left.In)
		rightRank := parameterLocationRank(right.In)
		if leftRank != rightRank {
			return leftRank < rightRank
		}

		leftName := strings.ToLower(strings.TrimSpace(left.Name))
		rightName := strings.ToLower(strings.TrimSpace(right.Name))
		if leftName != rightName {
			return leftName < rightName
		}

		return left.Name < right.Name
	})
}

func parameterLocationRank(location string) int {
	switch strings.ToLower(strings.TrimSpace(location)) {
	case "path":
		return 0
	case "query":
		return 1
	case "header":
		return 2
	case "cookie":
		return 3
	default:
		return 4
	}
}

func aclPermissionsRequireBearer(perms []string) bool {
	for _, perm := range perms {
		p := strings.ToLower(strings.TrimSpace(perm))
		if p == "" {
			continue
		}

		// ACL slug style (e.g. "order.read", "menu.write") implies authenticated actor.
		if strings.Contains(p, ".") {
			return true
		}

		if strings.Contains(p, "requires valid jwt token") ||
			strings.Contains(p, "requires systemadmin role") ||
			strings.Contains(p, "requires tenantadmin role") ||
			strings.Contains(p, "requires specific permissions") ||
			strings.Contains(p, "requires any of permissions") ||
			strings.Contains(p, "requires all permissions") {
			return true
		}
	}
	return false
}
