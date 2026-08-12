package annot8

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	typeIndex     *TypeIndex
	typeIndexOnce sync.Once
	modulePath    string // loaded from go.mod to identify internal packages
)

// Find a way to add method that will add external known types to the type index
// This is useful for types that are not defined in the current package but are known to the OpenAPI spec,
// such as types from external libraries or standard library types that we want to document.
func ensureTypeIndex() {
	// debug.PrintStack()
	typeIndexOnce.Do(func() {
		// load module path for package classification
		loadModulePath()
		slog.Debug("[annot8] cache.go: initializing typeIndex and externalKnownTypes")
		// Build type index once at startup
		typeIndex = BuildTypeIndex()
		// Log the number of types and files indexed
		slog.Debug(
			"[annot8] cache.go: typeIndex initialized",
			"types",
			len(typeIndex.types),
			"files",
			len(typeIndex.files),
		)
	})
}

// TypeIndex provides fast lookup of type definitions by package and type name.
type TypeIndex struct {
	types              map[string]map[string]*ast.TypeSpec // package -> type -> spec
	files              map[string]*ast.File                // file path -> parsed file
	externalKnownTypes map[string]*Schema                  // external known types
	qualifiedTypes     map[string]*ast.TypeSpec            // qualified type name -> spec (e.g., "order.CreateReq")
	packageImports     map[string]string                   // import path -> package name (e.g., "github.com/user/sqlc" -> "sqlc")
	loadedExternalPkgs map[string]bool                     // package alias -> attempted (to avoid repeated go list calls)
	typeJSONHints      map[string]typeJSONHint             // qualified type name -> marshaler interface hints
}

type typeJSONHint struct {
	hasMarshalJSON   bool
	hasUnmarshalJSON bool
	hasMarshalText   bool
	hasUnmarshalText bool
}

// BuildTypeIndex scans the given roots and builds a type index for all Go types.
func BuildTypeIndex() *TypeIndex {
	idx := &TypeIndex{
		types:              make(map[string]map[string]*ast.TypeSpec),
		files:              make(map[string]*ast.File),
		externalKnownTypes: make(map[string]*Schema),
		qualifiedTypes:     make(map[string]*ast.TypeSpec),
		packageImports:     make(map[string]string),
		loadedExternalPkgs: make(map[string]bool),
		typeJSONHints:      make(map[string]typeJSONHint),
	}

	// Find project root by looking for go.mod
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		slog.Debug("[annot8] BuildTypeIndex: could not find project root, using current directory")
		projectRoot = "."
	} else {
		slog.Debug("[annot8] BuildTypeIndex: using project root", "root", projectRoot)
	}

	_ = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil ||
			info.IsDir() ||
			!strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}

		return idx.indexFile(path)
	})

	slog.Debug("[annot8] BuildTypeIndex: completed", "totalPackages", len(idx.types), "totalFiles", len(idx.files))
	return idx
}

// indexFile processes a single Go file and indexes its types
func (idx *TypeIndex) indexFile(filePath string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		slog.Debug("[annot8] BuildTypeIndex: failed to parse file", "path", filePath, "err", err)
		return nil // Continue with other files
	}

	// Normalize path for consistent lookups across platforms
	normalizedPath := filepath.ToSlash(filePath)
	idx.files[normalizedPath] = file
	pkg := file.Name.Name

	// Record package imports for external vs internal classification
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil && imp.Name.Name != "" {
			alias = imp.Name.Name
		} else {
			alias = path.Base(importPath)
		}
		idx.packageImports[importPath] = alias
	}

	if _, ok := idx.types[pkg]; !ok {
		idx.types[pkg] = make(map[string]*ast.TypeSpec)
	}

	// Index type declarations
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, spec := range gd.Specs {
				if ts, isTypeSpec := spec.(*ast.TypeSpec); isTypeSpec {
					typeName := ts.Name.Name
					qualifiedName := idx.getQualifiedTypeName(pkg, typeName)

					// Store in both maps
					idx.types[pkg][typeName] = ts
					idx.qualifiedTypes[qualifiedName] = ts

					slog.Debug(
						"[annot8] BuildTypeIndex: indexed type",
						"package", pkg,
						"type", typeName,
						"qualified", qualifiedName,
						"file", filePath,
					)
				}
			}
		}

		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		idx.indexMethodHint(pkg, fd)
	}

	return nil
}

func (idx *TypeIndex) indexMethodHint(pkg string, fd *ast.FuncDecl) {
	if fd == nil || fd.Recv == nil || fd.Name == nil || len(fd.Recv.List) != 1 {
		return
	}

	receiverName := receiverTypeName(fd)
	if receiverName == "" {
		return
	}

	qualifiedName := idx.getQualifiedTypeName(pkg, receiverName)
	hint := idx.typeJSONHints[qualifiedName]

	switch fd.Name.Name {
	case "MarshalJSON":
		hint.hasMarshalJSON = true
	case "UnmarshalJSON":
		hint.hasUnmarshalJSON = true
	case "MarshalText":
		hint.hasMarshalText = true
	case "UnmarshalText":
		hint.hasUnmarshalText = true
	default:
		return
	}

	idx.typeJSONHints[qualifiedName] = hint
}

func GetTypeIndex() *TypeIndex {
	if typeIndex == nil {
		slog.Error("[annot8] GetTypeIndex: typeIndex is nil, building type index")
		typeIndex = BuildTypeIndex()
	} else {
		slog.Debug("[annot8] GetTypeIndex: returning existing typeIndex")
	}
	return typeIndex
}

// LookupType returns the TypeSpec for a given package and type name, or nil if not found.
func (idx *TypeIndex) LookupType(pkg, typeName string) *ast.TypeSpec {
	if idx == nil {
		return nil
	}
	if pkgTypes, ok := idx.types[pkg]; ok {
		return pkgTypes[typeName]
	}
	return nil
}

// LookupQualifiedType returns the TypeSpec for a qualified type name (e.g., "order.CreateReq")
func (idx *TypeIndex) LookupQualifiedType(qualifiedName string) *ast.TypeSpec {
	if idx == nil {
		return nil
	}
	return idx.qualifiedTypes[qualifiedName]
}

// LookupFile returns the AST for a given file path, handling normalization and case-insensitivity on Windows.
func (idx *TypeIndex) LookupFile(filePath string) *ast.File {
	if idx == nil {
		return nil
	}
	normalized := filepath.ToSlash(filePath)
	if f, ok := idx.files[normalized]; ok {
		return f
	}

	// Case-insensitive fallback for Windows
	for p, f := range idx.files {
		if strings.EqualFold(p, normalized) {
			return f
		}
	}
	return nil
}

// LookupUnqualifiedType searches for a type across all packages and returns the first match along with qualified name
func (idx *TypeIndex) LookupUnqualifiedType(typeName string) (*ast.TypeSpec, string) {
	if idx == nil {
		return nil, ""
	}

	// First check if it's a basic type
	if isBasicType(typeName) {
		return nil, ""
	}

	// Collect candidate packages that define this type. Map iteration order
	// is nondeterministic, so we gather and sort package names to be stable.
	var candidates []string
	for pkgName, pkgTypes := range idx.types {
		if _, exists := pkgTypes[typeName]; exists {
			candidates = append(candidates, pkgName)
		}
	}
	if len(candidates) == 0 {
		return nil, ""
	}

	sort.Strings(candidates)

	// Prefer internal (non-external) packages over external ones.
	for _, pkgName := range candidates {
		if !idx.isExternalPackage(pkgName) {
			typeSpec := idx.types[pkgName][typeName]
			qualifiedName := idx.getQualifiedTypeName(pkgName, typeName)
			return typeSpec, qualifiedName
		}
	}

	// No internal candidate found; return the first external candidate (deterministic due to sorting)
	pkgName := candidates[0]
	typeSpec := idx.types[pkgName][typeName]
	qualifiedName := idx.getQualifiedTypeName(pkgName, typeName)
	return typeSpec, qualifiedName
}

// GetQualifiedTypeName returns the appropriate qualified name for a type
func (idx *TypeIndex) GetQualifiedTypeName(typeName string) string {
	// If already qualified, return as-is
	if strings.Contains(typeName, ".") {
		return typeName
	}

	// Look up the type and return its qualified name
	if _, qualifiedName := idx.LookupUnqualifiedType(typeName); qualifiedName != "" {
		return qualifiedName
	}

	// Fallback to original name
	return typeName
}

func AddExternalKnownType(name string, schema *Schema) {
	ensureTypeIndex() // Ensure typeIndex is initialized
	if typeIndex == nil {
		slog.Error("[annot8] AddExternalKnownType: typeIndex is nil, cannot add external type", "name", name)
		return
	}
	if typeIndex.externalKnownTypes == nil {
		typeIndex.externalKnownTypes = make(map[string]*Schema)
	}
	typeIndex.externalKnownTypes[name] = schema
	slog.Debug("[annot8] AddExternalKnownType: added external known type", "name", name)
}

// SetExternalKnownTypes replaces all configured external known type mappings.
func SetExternalKnownTypes(schemas map[string]*Schema) {
	ensureTypeIndex()
	if typeIndex == nil {
		slog.Error("[annot8] SetExternalKnownTypes: typeIndex is nil")
		return
	}
	typeIndex.SetExternalKnownTypes(schemas)
}

// AddExternalKnownTypes merges mappings into configured external known types.
func AddExternalKnownTypes(schemas map[string]*Schema) {
	ensureTypeIndex()
	if typeIndex == nil {
		slog.Error("[annot8] AddExternalKnownTypes: typeIndex is nil")
		return
	}
	typeIndex.AddExternalKnownTypes(schemas)
}

// SetExternalKnownTypes replaces all configured external known type mappings for this index.
func (idx *TypeIndex) SetExternalKnownTypes(schemas map[string]*Schema) {
	if idx == nil {
		return
	}
	idx.externalKnownTypes = make(map[string]*Schema, len(schemas))
	for name, schema := range schemas {
		idx.externalKnownTypes[name] = schema
	}
}

// AddExternalKnownTypes merges mappings into configured external known types for this index.
func (idx *TypeIndex) AddExternalKnownTypes(schemas map[string]*Schema) {
	if idx == nil || len(schemas) == 0 {
		return
	}
	if idx.externalKnownTypes == nil {
		idx.externalKnownTypes = make(map[string]*Schema, len(schemas))
	}
	for name, schema := range schemas {
		idx.externalKnownTypes[name] = schema
	}
}

func (idx *TypeIndex) inferMarshalerSchema(qualifiedName string) *Schema {
	if idx == nil {
		return nil
	}

	baseQualified := strings.TrimLeft(qualifiedName, "*")
	alias := externalPackageAlias(baseQualified)
	if alias != "" {
		_ = idx.loadExternalPackage(alias)
	}

	hint, ok := idx.typeJSONHints[baseQualified]
	if !ok {
		return nil
	}
	if !hint.hasMarshalJSON && !hint.hasUnmarshalJSON && !hint.hasMarshalText && !hint.hasUnmarshalText {
		return nil
	}

	if hint.hasMarshalText || hint.hasUnmarshalText {
		s := &Schema{Type: "string", Description: marshalerDescription(baseQualified, "text")}
		if format := inferStringFormat(baseQualified); format != "" {
			s.Format = format
		}
		return s
	}

	if s := idx.inferPrimitiveAliasSchema(baseQualified); s != nil {
		if hasType(s, "string") && s.Format == "" {
			s.Format = inferStringFormat(baseQualified)
		}
		if s.Description == "" {
			s.Description = marshalerDescription(baseQualified, "json")
		}
		return s
	}

	if format := inferStringFormat(baseQualified); format != "" {
		return &Schema{
			Type:        "string",
			Format:      format,
			Description: marshalerDescription(baseQualified, "json"),
		}
	}

	return &Schema{
		Type:        "object",
		Description: marshalerDescription(baseQualified, "json"),
	}
}

func marshalerDescription(qualifiedName, mode string) string {
	switch mode {
	case "text":
		return "Serialized as a JSON string via " + qualifiedName + " text marshaling methods"
	default:
		return "Serialized via " + qualifiedName + " JSON marshaling methods"
	}
}

func (idx *TypeIndex) inferPrimitiveAliasSchema(qualifiedName string) *Schema {
	ts := idx.LookupQualifiedType(qualifiedName)
	if ts == nil {
		return nil
	}

	ident, ok := ts.Type.(*ast.Ident)
	if !ok {
		return nil
	}

	openapiType, openapiFormat := mapGoTypeToOpenAPI(ident.Name)
	if openapiType == "object" {
		return nil
	}

	s := &Schema{Type: openapiType}
	if openapiFormat != "" {
		s.Format = openapiFormat
	}
	return s
}

func inferStringFormat(qualifiedName string) string {
	name := strings.TrimLeft(qualifiedName, "*")
	if dot := strings.LastIndex(name, "."); dot != -1 {
		name = name[dot+1:]
	}

	tokens := tokenizeTypeName(name)
	has := func(token string) bool {
		for _, t := range tokens {
			if t == token {
				return true
			}
		}
		return false
	}

	switch {
	case has("uuid"):
		return "uuid"
	case has("email"):
		return "email"
	case has("uri") || has("url"):
		return "uri"
	case has("ipv6"):
		return "ipv6"
	case has("ipv4"):
		return "ipv4"
	case has("duration"):
		return "duration"
	case has("timestamp") || has("datetime") || has("time"):
		return "date-time"
	case has("date"):
		return "date"
	default:
		return ""
	}
}

func tokenizeTypeName(s string) []string {
	var tokens []string
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(string(current)))
		current = current[:0]
	}

	runes := []rune(s)
	var prev rune
	for i, r := range runes {
		if r == '_' || r == '-' || r == '.' || r == ' ' {
			flush()
			prev = r
			continue
		}
		if i > 0 && isASCIIUpper(r) {
			if isASCIILower(prev) || isASCIIDigit(prev) {
				flush()
			} else if i+1 < len(runes) && isASCIILower(runes[i+1]) {
				// Split acronym-to-word boundary: UUIDText -> UUID + Text.
				flush()
			}
		}
		current = append(current, r)
		prev = r
	}
	flush()

	return tokens
}

func isASCIIUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isASCIILower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// getQualifiedTypeName creates a qualified type name for indexing.
// For external packages (like sqlc, pgtype), use the package name as-is.
// For internal project types, use package.TypeName format.
func (idx *TypeIndex) getQualifiedTypeName(pkg, typeName string) string {
	// Check if this is an external/third-party package
	if idx.isExternalPackage(pkg) {
		return pkg + "." + typeName
	}

	// For internal project types, use package.TypeName format
	return pkg + "." + typeName
}

// isExternalPackage determines if a package is external/third-party
func (idx *TypeIndex) isExternalPackage(pkg string) bool {
	// If an import alias maps to a path outside the current module, treat as external
	for importPath, alias := range idx.packageImports {
		if alias == pkg {
			if modulePath != "" && strings.HasPrefix(importPath, modulePath) {
				return false
			}
			return true
		}
	}
	// Default to internal
	return false
}

// loadExternalPackage finds the source directory for the external package identified by pkgAlias
// (e.g. "pgtype"), parses its Go files, and indexes the discovered types.
// Returns true if the package was successfully located and indexed.
// Each alias is attempted at most once; subsequent calls are no-ops.
func (idx *TypeIndex) loadExternalPackage(pkgAlias string) bool {
	if idx.loadedExternalPkgs[pkgAlias] {
		return false // already attempted
	}
	idx.loadedExternalPkgs[pkgAlias] = true

	// Reverse-lookup: find the full import path for this alias.
	importPath := ""
	for ip, alias := range idx.packageImports {
		if alias == pkgAlias {
			importPath = ip
			break
		}
	}
	if importPath == "" {
		slog.Debug("[annot8] loadExternalPackage: no import path found for alias", "alias", pkgAlias)
		return false
	}

	// Ask the Go toolchain for the source directory of this package.
	projectRoot := findProjectRoot()
	cmd := exec.Command("go", "list", "-json", importPath)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	out, err := cmd.Output()
	if err != nil {
		slog.Debug("[annot8] loadExternalPackage: go list failed", "importPath", importPath, "err", err)
		return false
	}

	var info struct {
		Dir string `json:"Dir"`
	}
	if err = json.Unmarshal(out, &info); err != nil || info.Dir == "" {
		slog.Debug("[annot8] loadExternalPackage: could not parse go list output", "importPath", importPath)
		return false
	}

	// Index every non-test .go file in the directory.
	entries, err := os.ReadDir(info.Dir)
	if err != nil {
		slog.Debug("[annot8] loadExternalPackage: could not read dir", "dir", info.Dir, "err", err)
		return false
	}

	indexed := 0
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			if indexErr := idx.indexFile(filepath.Join(info.Dir, name)); indexErr == nil {
				indexed++
			}
		}
	}

	slog.Debug("[annot8] loadExternalPackage: indexed external package",
		"alias", pkgAlias, "importPath", importPath, "dir", info.Dir, "files", indexed)
	return indexed > 0
}

// findProjectRoot finds the project root by looking for go.mod file
func findProjectRoot() string {
	// Start from current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Walk up the directory tree looking for go.mod
	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err = os.Stat(goModPath); err == nil {
			return currentDir
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parentDir
	}

	return ""
}

// loadModulePath reads the Go module path from go.mod to distinguish internal vs external packages
func loadModulePath() {
	if modulePath != "" {
		return
	}
	root := findProjectRoot()
	if root == "" {
		return
	}
	// #nosec G304 -- build-time codegen tool; root is this module's own root as
	// located by findProjectRoot, and the filename is the literal "go.mod".
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			return
		}
	}
}
