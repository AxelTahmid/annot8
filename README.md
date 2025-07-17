# Chi OpenApi Gen

![ChiOpenApiGen](./banner.png)

An annotation-driven OpenAPI 3.1 specification generator for Go HTTP services using Chi router. This package automatically generates comprehensive API documentation from your Go code with zero configuration required.

**About This Project**: This project was initially part of a larger monolith, where we needed immediate documentation. It was using Chi and sqlc with pgx/v5, therefore, this package would have the best support for that tech stack only. If this fits your usecase, feel free to use it. There are a lot of rooms for improvements, but it serves the purpose necessary for us at this point of time. Contributions are welcome, see the [CONTRIBUTING.md](CONTRIBUTING.md) file. This readme.md is AI generated.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/AxelTahmid/openapi-gen)](https://goreportcard.com/report/github.com/AxelTahmid/openapi-gen)

## Features

-   **Zero Configuration**: No manual type registration or complex setup required
-   **Chi Router Native**: Specifically designed and optimized for `go-chi/chi` router
-   **Annotation-Driven**: Uses standard Swagger-style comments for documentation
-   **Dynamic Schema Generation**: Automatically generates JSON schemas from Go types
-   **High Performance**: Built-in caching and type indexing for optimal performance
-   **Type Safety**: Leverages Go's type system for accurate schema generation
-   **Deep Type Discovery**: Recursively finds and documents all referenced types
-   **External Type Support**: Configurable support for third-party library types
-   **Runtime Generation**: Updates documentation dynamically without restarts

## Current Limitations

As mentioned, this package was extracted from a larger project and has room for improvements:

-   **Chi Router Only**: Currently only supports `go-chi/chi` router (by design)
-   **Top-Level Functions**: Handlers must be top-level functions, not struct methods
-   **SQLC/pgx Optimized**: Best performance with SQLC-generated types and pgx/v5
-   **AST Parsing Limitations**: Complex comment patterns may not be parsed correctly
-   **Limited Router Support**: No plans to support other routers (Gin, Echo, etc.)
-   **Type Discovery**: May struggle with deeply nested or complex generic types
-   **Documentation**: Some edge cases in annotation parsing may need manual workarounds

Despite these limitations, the package serves its core purpose effectively for Chi + SQLC + pgx/v5 projects.

## Installation

```bash
go get github.com/AxelTahmid/openapi-gen
```

## Quick Start

### 1. Define Your Types

```go
// User represents a user in the system
type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     *string   `json:"email,omitempty"`
    Age       int       `json:"age"`
    IsActive  bool      `json:"is_active"`
    CreatedAt time.Time `json:"created_at"`
    Tags      []string  `json:"tags"`
}

// CreateUserRequest represents the request payload for creating a user
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

// UserListResponse represents a paginated list of users
type UserListResponse struct {
    Users []User `json:"users"`
    Total int    `json:"total"`
    Page  int    `json:"page"`
    Limit int    `json:"limit"`
}
```

### 2. Create Annotated Handler Functions

**Important**: Handlers must be **top-level functions**, not struct methods, for annotation parsing to work correctly.

```go
// GetUsers retrieves a paginated list of users
// @Summary Get all users
// @Description Retrieve a paginated list of users with optional filtering
// @Tags users
// @Accept application/json
// @Produce application/json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param active query bool false "Filter by active status"
// @Success 200 {object} UserListResponse "List of users"
// @Failure 400 {object} ProblemDetails "Invalid request parameters"
// @Failure 500 {object} ProblemDetails "Internal server error"
func GetUsers(w http.ResponseWriter, r *http.Request) {
    // Implementation here
    users := UserListResponse{
        Users: []User{{ID: 1, Name: "John Doe", Age: 30}},
        Total: 1,
        Page:  1,
        Limit: 10,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

// CreateUser creates a new user in the system
// @Summary Create a new user
// @Description Create a new user with the provided details
// @Tags users
// @Accept application/json
// @Produce application/json
// @Param user body CreateUserRequest true "User creation data"
// @Success 201 {object} User "User created successfully"
// @Failure 400 {object} ProblemDetails "Invalid request data"
// @Failure 409 {object} ProblemDetails "User already exists"
// @Failure 500 {object} ProblemDetails "Internal server error"
// @Security BearerAuth
func CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation here
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    user := User{
        ID:   2,
        Name: req.Name,
        Age:  req.Age,
        // ... other fields
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}
```

### 3. Setup Router and OpenAPI Endpoints

#### Option A: Integrated with Your API Server

```go
package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/AxelTahmid/openapi-gen"
)

func main() {
    r := chi.NewRouter()

    // Add middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Configure OpenAPI
    config := openapi.Config{
        Title:          "User Management API",
        Description:    "A comprehensive API for managing users",
        Version:        "1.0.0",
        TermsOfService: "https://example.com/terms",
        Server:         "https://api.example.com",
        Contact: &openapi.Contact{
            Name:  "API Support Team",
            Email: "api-support@example.com",
            URL:   "https://example.com/support",
        },
        License: &openapi.License{
            Name: "Apache 2.0",
            URL:  "https://www.apache.org/licenses/LICENSE-2.0.html",
        },
    }

    // Add OpenAPI endpoints (typically only in development)
    r.Route("/openapi", func(r chi.Router) {
        r.Get("/", openapi.CachedHandler(r, config))
        r.Get("/generate", openapi.GenerateFileHandler(r, config))
        r.Get("/invalidate", openapi.InvalidateCache)
    })

    // Add your API routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/users", GetUsers)
        r.Post("/users", CreateUser)
        r.Get("/users/{id}", GetUserByID)
        r.Put("/users/{id}", UpdateUser)
        r.Delete("/users/{id}", DeleteUser)
    })

    log.Println("Server starting on :8080")
    log.Println("OpenAPI spec available at: http://localhost:8080/openapi")
    http.ListenAndServe(":8080", r)
}
```

#### Option B: Standalone CLI Tool for File Generation

```go
// cmd/generate-docs/main.go
package main

import (
    "log"
    "os"

    "github.com/go-chi/chi/v5"
    "github.com/AxelTahmid/openapi-gen"
)

func main() {
    // Create router and register routes (same as above)
    r := chi.NewRouter()
    r.Get("/api/v1/users", GetUsers)
    r.Post("/api/v1/users", CreateUser)
    // ... other routes

    config := openapi.Config{
        Title:   "User Management API",
        Version: "1.0.0",
        Server:  "https://api.example.com",
    }

    // Generate OpenAPI spec file
    err := openapi.GenerateOpenAPISpecFile(r, config, "openapi.json", true)
    if err != nil {
        log.Fatalf("Failed to generate OpenAPI spec: %v", err)
    }

    log.Println("OpenAPI specification generated successfully: openapi.json")
}
```

### 4. Access Your Documentation

```bash
# View the OpenAPI specification
curl http://localhost:8080/openapi | jq .

# Use with Swagger UI
docker run -p 8080:8080 -e SWAGGER_JSON_URL=http://host.docker.internal:8080/openapi swaggerapi/swagger-ui

# Generate static file
curl http://localhost:8080/openapi/generate

# Invalidate cache for fresh generation
curl http://localhost:8080/openapi/invalidate
```

## Usage Scenarios

This package supports various deployment and usage patterns. Choose the approach that best fits your project:

### Development Mode (Runtime Generation)

Perfect for local development and staging environments:

```go
func main() {
    r := chi.NewRouter()

    // Add your API routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/users", GetUsers)
        r.Post("/users", CreateUser)
    })

    // Add OpenAPI endpoints for development
    if os.Getenv("ENV") != "production" {
        config := openapi.Config{
            Title:   "My API",
            Version: "1.0.0",
            Server:  "http://localhost:8080",
        }

        r.Route("/docs", func(r chi.Router) {
            r.Get("/openapi.json", openapi.CachedHandler(r, config))
            r.Get("/", swaggerUIHandler) // Your Swagger UI handler
        })
    }

    log.Println("API docs: http://localhost:8080/docs")
    http.ListenAndServe(":8080", r)
}
```

### Production Mode (Static File Generation)

Generate static OpenAPI files for production deployment:

```go
// cmd/generate-docs/main.go
package main

import (
    "log"
    "github.com/AxelTahmid/openapi-gen"
)

func main() {
    // Setup your router with all routes
    r := setupProductionRoutes()

    config := openapi.Config{
        Title:       "Production API",
        Version:     "1.0.0",
        Server:      "https://api.yourdomain.com",
        Description: "Production API documentation",
    }

    // Generate static file
    err := openapi.GenerateOpenAPISpecFile(r, config, "docs/openapi.json", true)
    if err != nil {
        log.Fatalf("Failed to generate spec: %v", err)
    }

    log.Println("Static OpenAPI spec generated at docs/openapi.json")
}
```

### CI/CD Integration

Integrate documentation generation into your build pipeline:

```yaml
# .github/workflows/docs.yml
name: Generate API Documentation

on:
    push:
        branches: [main]
    pull_request:
        branches: [main]

jobs:
    generate-docs:
        runs-on: ubuntu-latest
        steps:
            - uses: actions/checkout@v3

            - name: Setup Go
              uses: actions/setup-go@v3
              with:
                  go-version: '1.24'

            - name: Generate OpenAPI spec
              run: |
                  go run cmd/generate-docs/main.go

            - name: Validate OpenAPI spec
              run: |
                  npx @apidevtools/swagger-cli validate docs/openapi.json

            - name: Deploy to GitHub Pages
              if: github.ref == 'refs/heads/main'
              uses: peaceiris/actions-gh-pages@v3
              with:
                  github_token: ${{ secrets.GITHUB_TOKEN }}
                  publish_dir: ./docs
```

### Testing Integration

Use in your test suites for API contract testing:

```go
func TestAPIDocumentation(t *testing.T) {
    // Setup test router
    r := setupTestRoutes()

    config := openapi.Config{
        Title:   "Test API",
        Version: "1.0.0",
    }

    // Generate spec
    handler := openapi.CachedHandler(r, config)
    req := httptest.NewRequest("GET", "/openapi", nil)
    w := httptest.NewRecorder()
    handler(w, req)

    // Validate spec structure
    var spec openapi.Spec
    err := json.Unmarshal(w.Body.Bytes(), &spec)
    require.NoError(t, err)

    // Test specific endpoints are documented
    assert.Contains(t, spec.Paths, "/api/v1/users")
    assert.Equal(t, "Test API", spec.Info.Title)
}

func TestAPIContractCompliance(t *testing.T) {
    // Ensure your API responses match the generated schema
    // Use tools like go-swagger validator or custom validation
}
```

### Microservices Architecture

Generate documentation for multiple services:

```go
// Service A
func setupUserService() *chi.Mux {
    r := chi.NewRouter()
    r.Route("/api/v1/users", func(r chi.Router) {
        r.Get("/", GetUsers)
        r.Post("/", CreateUser)
    })
    return r
}

// Service B
func setupOrderService() *chi.Mux {
    r := chi.NewRouter()
    r.Route("/api/v1/orders", func(r chi.Router) {
        r.Get("/", GetOrders)
        r.Post("/", CreateOrder)
    })
    return r
}

// Generate separate specs for each service
func generateServiceDocs() {
    services := map[string]*chi.Mux{
        "user-service":  setupUserService(),
        "order-service": setupOrderService(),
    }

    for name, router := range services {
        config := openapi.Config{
            Title:   fmt.Sprintf("%s API", strings.Title(name)),
            Version: "1.0.0",
            Server:  fmt.Sprintf("https://%s.api.company.com", name),
        }

        filename := fmt.Sprintf("docs/%s-openapi.json", name)
        err := openapi.GenerateOpenAPISpecFile(router, config, filename, true)
        if err != nil {
            log.Printf("Failed to generate %s docs: %v", name, err)
        }
    }
}
```

### Custom Middleware Integration

Works seamlessly with Chi middleware:

```go
func main() {
    r := chi.NewRouter()

    // Global middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.RequestID)
    r.Use(corsMiddleware)

    // API routes with specific middleware
    r.Route("/api/v1", func(r chi.Router) {
        r.Use(authMiddleware)      // JWT authentication
        r.Use(rateLimitMiddleware) // Rate limiting
        r.Use(metricsMiddleware)   // Prometheus metrics

        r.Get("/users", GetUsers)
        r.Post("/users", CreateUser)
    })

    // Documentation routes (no auth required)
    r.Route("/docs", func(r chi.Router) {
        r.Use(middleware.BasicAuth("docs", map[string]string{
            "admin": "secret", // Basic auth for docs
        }))

        config := openapi.Config{
            Title:   "Protected API",
            Version: "1.0.0",
        }

        r.Get("/openapi.json", openapi.CachedHandler(r, config))
    })

    http.ListenAndServe(":8080", r)
}
```

## Why Top-Level Functions?

This package uses Go's AST parsing to extract function comments and annotations. For the parser to correctly identify and process your handler functions, they must be defined as **top-level functions** rather than struct methods.

### Won't Work (Struct Methods)

```go
type UserHandler struct {
    service UserService
}

// This annotation won't be parsed correctly
// @Summary Create user
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### Will Work (Top-Level Functions)

```go
// This will be found and parsed correctly
// @Summary Create user
// @Description Create a new user with the provided details
func CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// You can still use dependency injection patterns
func CreateUserWithService(service UserService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Implementation using service
    }
}
```

## Supported Annotations

| Annotation     | Format                                                 | Description                   | Example                                                    |
| -------------- | ------------------------------------------------------ | ----------------------------- | ---------------------------------------------------------- |
| `@Summary`     | `@Summary <text>`                                      | Brief endpoint description    | `@Summary Create a new user`                               |
| `@Description` | `@Description <text>`                                  | Detailed endpoint description | `@Description Create a new user with the provided details` |
| `@Tags`        | `@Tags <tag1>,<tag2>`                                  | Comma-separated list of tags  | `@Tags users,management`                                   |
| `@Accept`      | `@Accept <media-type>`                                 | Request content type          | `@Accept application/json`                                 |
| `@Produce`     | `@Produce <media-type>`                                | Response content type         | `@Produce application/json`                                |
| `@Param`       | `@Param <name> <in> <type> <required> "<description>"` | Request parameters            | See examples below                                         |
| `@Success`     | `@Success <code> {<format>} <type> "<description>"`    | Success responses             | `@Success 200 {object} User "Success"`                     |
| `@Failure`     | `@Failure <code> {<format>} <type> "<description>"`    | Error responses               | `@Failure 400 {object} ProblemDetails "Bad Request"`       |
| `@Security`    | `@Security <scheme>`                                   | Security requirements         | `@Security BearerAuth`                                     |

### Parameter Types (`@Param`)

| Store    | Example                                                | Description        |
| -------- | ------------------------------------------------------ | ------------------ |
| `body`   | `@Param user body CreateUserRequest true "User data"`  | Request body       |
| `path`   | `@Param id path int true "User ID"`                    | URL path parameter |
| `query`  | `@Param limit query int false "Page limit"`            | Query parameter    |
| `header` | `@Param Authorization header string true "Auth token"` | Header parameter   |

### Response Formats (`@Success` / `@Failure`)

| Format     | Example                                     | Description      |
| ---------- | ------------------------------------------- | ---------------- |
| `{object}` | `@Success 200 {object} User "Single user"`  | Single object    |
| `{array}`  | `@Success 200 {array} User "List of users"` | Array of objects |
| `{data}`   | `@Success 200 {data} string "Raw data"`     | Raw data type    |

## Advanced Configuration

### Full Configuration Example

```go
config := openapi.Config{
    Title:          "E-Commerce API",
    Description:    "Comprehensive REST API for e-commerce operations",
    Version:        "2.1.0",
    TermsOfService: "https://example.com/terms",
    Server:         "https://api.example.com",
    Contact: &openapi.Contact{
        Name:  "E-Commerce API Team",
        Email: "api-team@example.com",
        URL:   "https://example.com/support",
    },
    License: &openapi.License{
        Name: "Apache 2.0",
        URL:  "https://www.apache.org/licenses/LICENSE-2.0.html",
    },
}
```

### Adding External Type Mappings

```go
// Add support for custom types from external libraries
openapi.AddExternalKnownType("github.com/shopspring/decimal.Decimal", &openapi.Schema{
    Type:        "string",
    Description: "Decimal number represented as string",
    Example:     "123.45",
})

openapi.AddExternalKnownType("github.com/google/uuid.UUID", &openapi.Schema{
    Type:        "string",
    Format:      "uuid",
    Description: "UUID v4",
    Example:     "550e8400-e29b-41d4-a716-446655440000",
})
```

## Schema Generation

The package automatically generates JSON schemas for your Go types with the following features:

### Features

-   **Automatic Discovery**: Finds types by scanning your project files
-   **Package-Aware**: Supports both local types (`User`) and package-qualified types (`db.User`)
-   **Struct Tag Support**: Respects `json` tags and `omitempty` directives
-   **Type Mapping**: Maps Go types to appropriate OpenAPI types
-   **Reference Resolution**: Handles circular references and type reuse
-   **Performance Optimized**: Built-in type indexing and caching

### Type Discovery Process

1. **Current Package**: For unqualified types like `CreateUserRequest`
2. **Project Packages**: Recursively searches under project directories
3. **Package-Qualified**: For types like `db.User` or `models.Product`
4. **External Types**: Configurable mappings for third-party types

### Example Schema Generation

Given this Go struct:

```go
type Product struct {
    ID          int                    `json:"id"`
    Name        string                 `json:"name"`
    Description *string                `json:"description,omitempty"`
    Price       float64                `json:"price"`
    InStock     bool                   `json:"in_stock"`
    Tags        []string               `json:"tags"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    CreatedAt   time.Time              `json:"created_at"`
    Category    Category               `json:"category"`
}
```

The generator produces this OpenAPI schema:

```json
{
    "type": "object",
    "properties": {
        "id": { "type": "integer" },
        "name": { "type": "string" },
        "description": { "type": "string" },
        "price": { "type": "number" },
        "in_stock": { "type": "boolean" },
        "tags": {
            "type": "array",
            "items": { "type": "string" }
        },
        "metadata": { "type": "object" },
        "created_at": { "type": "string", "format": "date-time" },
        "category": { "$ref": "#/components/schemas/Category" }
    },
    "required": [
        "id",
        "name",
        "price",
        "in_stock",
        "tags",
        "created_at",
        "category"
    ]
}
```

## Security Integration

The package automatically detects security requirements and generates appropriate security schemes:

```go
// Protected endpoint
// @Security BearerAuth
func CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// Multiple security schemes
// @Security BearerAuth
// @Security ApiKeyAuth
func AdminOnlyEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## Integration Examples

### With Authentication Middleware

```go
r.Route("/api/v1", func(r chi.Router) {
    // Public routes
    r.Post("/auth/login", LoginUser)
    r.Post("/auth/register", RegisterUser)

    // Protected routes (will automatically include BearerAuth requirement)
    r.Group(func(r chi.Router) {
        r.Use(authMiddleware) // JWT middleware
        r.Get("/users", ListUsers)
        r.Post("/users", CreateUser)
    })
})
```

### Multiple API Versions

```go
// v1 routes
r.Route("/api/v1", func(r chi.Router) {
    r.Get("/users", V1ListUsers)
    r.Post("/users", V1CreateUser)
})

// v2 routes
r.Route("/api/v2", func(r chi.Router) {
    r.Get("/users", V2ListUsers)
    r.Post("/users", V2CreateUser)
})

// Separate OpenAPI specs for each version
r.Get("/api/v1/openapi.json", openapi.CachedHandler(r, v1Config))
r.Get("/api/v2/openapi.json", openapi.CachedHandler(r, v2Config))
```

### Error Handling Best Practices

```go
// Define standard error response
type ProblemDetails struct {
    Type     string `json:"type"`
    Title    string `json:"title"`
    Status   int    `json:"status"`
    Detail   string `json:"detail,omitempty"`
    Instance string `json:"instance,omitempty"`
}

// Use consistent error responses
// @Failure 400 {object} ProblemDetails "Bad request"
// @Failure 401 {object} ProblemDetails "Unauthorized"
// @Failure 403 {object} ProblemDetails "Forbidden"
// @Failure 404 {object} ProblemDetails "Not found"
// @Failure 500 {object} ProblemDetails "Internal server error"
func MyHandler(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## Testing Your OpenAPI Spec

### Validate Generated Spec

```bash
# Test the endpoint
curl http://localhost:8080/openapi | jq .

# Validate with swagger-codegen
npx @apidevtools/swagger-cli validate http://localhost:8080/openapi

# Generate client code
npx @openapitools/openapi-generator-cli generate \
  -i http://localhost:8080/openapi \
  -g typescript-fetch \
  -o ./generated-client
```

### Integration with Swagger UI

```bash
# Run Swagger UI with Docker
docker run -p 8080:8080 \
  -e SWAGGER_JSON_URL=http://host.docker.internal:3000/openapi \
  swaggerapi/swagger-ui

# Or with docker-compose
version: '3.8'
services:
  swagger-ui:
    image: swaggerapi/swagger-ui
    ports:
      - "8080:8080"
    environment:
      SWAGGER_JSON_URL: http://host.docker.internal:3000/openapi
```

### Automated Testing

```go
func TestOpenAPISpec(t *testing.T) {
    router := setupTestRouter()
    config := openapi.Config{
        Title:   "Test API",
        Version: "1.0.0",
    }

    handler := openapi.CachedHandler(router, config)

    req := httptest.NewRequest("GET", "/openapi", nil)
    w := httptest.NewRecorder()
    handler(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var spec openapi.Spec
    err := json.Unmarshal(w.Body.Bytes(), &spec)
    assert.NoError(t, err)
    assert.Equal(t, "3.1.0", spec.OpenAPI)
    assert.Equal(t, "Test API", spec.Info.Title)
}
```

## Performance & Caching

The package includes several performance optimizations:

-   **Type Index Caching**: Built-in type discovery cache
-   **Spec Caching**: Generated specifications are cached
-   **Smart Invalidation**: Cache invalidation when needed
-   **Lazy Loading**: Types are discovered and parsed on-demand

### Cache Management

```go
// Invalidate cache programmatically
openapi.InvalidateCache()

// Force refresh via HTTP
curl http://localhost:8080/openapi?refresh=true

// Invalidate via endpoint
curl http://localhost:8080/openapi/invalidate
```

## Architecture

The package consists of several main components:

| Component            | Purpose                                    | Key Features                                 |
| -------------------- | ------------------------------------------ | -------------------------------------------- |
| **Generator**        | Main specification generator               | Route walking, operation building            |
| **Router Discovery** | Chi router analysis and route extraction   | Route introspection, handler identification  |
| **Annotations**      | Comment parsing and annotation extraction  | Swagger annotation support, error reporting  |
| **Schema Generator** | Dynamic Go type to JSON schema conversion  | Type discovery, recursive generation         |
| **Cache**            | Type indexing and performance optimization | AST caching, type lookup, smart invalidation |

## Common Issues & Solutions

### Issue: Annotations Not Being Parsed

**Problem**: Handler annotations are ignored.

**Solution**: Ensure handlers are top-level functions, not struct methods.

```go
// Wrong
type Handler struct{}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {}

// Correct
func Create(w http.ResponseWriter, r *http.Request) {}
```

### Issue: Types Not Found

**Problem**: Custom types not appearing in generated schemas.

**Solution**: Ensure types are in the same project or add external mappings.

```go
// Add external type mapping
openapi.AddExternalKnownType("external.Type", &openapi.Schema{
    Type: "object",
    Description: "External type description",
})
```

### Issue: Circular References

**Problem**: Infinite recursion with self-referencing types.

**Solution**: The package handles this automatically with reference cycles.

### Issue: Performance in Large Projects

**Problem**: Slow generation with many types.

**Solution**: Use the built-in caching and consider pre-building type index.

## Development & Contributing

### Prerequisites

-   Go 1.24 or higher
-   Git

### Setting Up Development Environment

```bash
# Clone the repository
git clone https://github.com/AxelTahmid/openapi-gen.git
cd openapi-gen

# Install dependencies
go mod download

# Run tests
go test ./...

# Run tests with coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./...

# Lint code
golangci-lint run
```

### Project Structure

```
openapi-gen/
├── annotations.go              # Annotation parsing and validation
├── annotations_test.go         # Annotation parsing tests
├── cache.go                    # Type indexing and caching system
├── generator.go                # Core OpenAPI specification generator
├── generator_spec_test.go      # Generator integration tests
├── handlers.go                 # HTTP handlers for serving specs
├── openapi_test.go             # OpenAPI generation tests
├── qualified_names_test.go     # Type name resolution tests
├── router_discovery.go         # Chi router route discovery
├── router_discovery_test.go    # Router discovery tests
├── schema.go                   # Core schema generation logic
├── schema_basic_types.go       # Basic Go type mappings
├── schema_enums.go             # Enum type handling
├── schema_structs.go           # Struct schema generation
├── schema_tags.go              # JSON tag processing
├── schema_test.go              # Schema generation tests
├── test_helpers.go             # Test utilities and helpers
└── README.md                   # This file
```

### Running Tests

```bash
# Run all tests
make test-openapi

# Run with verbose output
make test-openapi-verbose

# Run specific test
go test -run TestGenerator_GenerateSpec ./pkg/openapi

# Run benchmarks
go test -bench=BenchmarkGenerateSpec ./pkg/openapi
```

### Contributing Guidelines

See the [CONTRIBUTING.md](CONTRIBUTING.md).

### Adding New Features

When adding new features:

1. Add comprehensive tests
2. Update documentation and examples
3. Consider backward compatibility
4. Add benchmarks for performance-critical code
5. Update the changelog

### Code Style

-   Follow standard Go formatting (`gofmt`)
-   Use meaningful variable and function names
-   Add documentation for exported functions
-   Keep functions focused and small
-   Use structured logging with appropriate levels

## Roadmap

### Immediate Improvements (Community Contributions Welcome)

-   [ ] **Better Error Messages**: Improve AST parsing error reporting and debugging
-   [ ] **Enhanced Type Support**: Better support for generics and complex nested types
-   [ ] **Documentation**: More examples and edge case handling
-   [ ] **Performance**: Optimize type discovery for large codebases
-   [ ] **Testing**: Expand test coverage for edge cases

### Future Possibilities (Not Guaranteed)

-   [ ] **OpenAPI 3.1 Full Compliance**: Complete OpenAPI 3.1 specification support
-   [ ] **Validation Integration**: Runtime request/response validation
-   [ ] **Mock Server Generation**: Generate mock servers from specs
-   [ ] **Better SQLC Integration**: Native support for more SQLC patterns

### Explicitly Not Planned

-   **Multiple Router Support**: This package is Chi-specific by design
-   **GraphQL Support**: Out of scope for this project
-   **Client Generation**: Use existing OpenAPI client generators
-   **Struct Method Support**: AST limitations make this impractical

**Note**: This project serves a specific need (Chi + SQLC + pgx/v5). Major feature additions should align with this core use case. For other routers or frameworks, consider using more general-purpose OpenAPI generators.

## Changelog

See the [CHANGELOG.md](CHANGELOG.md).

## Support & Community

### Getting Help

-   **Documentation**: Check this README and code examples
-   **Bug Reports**: Open an issue on GitHub with detailed reproduction steps
-   **Feature Requests**: Open an issue with the `enhancement` label
-   **Questions**: Start a discussion in the repository discussions

### Community Resources

-   **GitHub Issues**: Bug reports and feature requests
-   **GitHub Discussions**: Questions and community support
-   **Examples Repository**: Real-world usage examples
-   **Blog Posts**: Tutorials and best practices

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

-   **Chi Router Team**: For the excellent HTTP router
-   **OpenAPI Initiative**: For the comprehensive API specification standard
-   **Go Community**: For the amazing ecosystem and tools
-   **Contributors**: Everyone who has contributed to this project

---

**Made for the Go community**

If this project helps you, please consider giving it a star on GitHub!
