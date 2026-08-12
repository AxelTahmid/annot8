package annot8

import (
	"net/http"
	"reflect"
	"runtime"
	"strings"
)

// MiddlewareSecurityRule maps a middleware name pattern to one or more
// OpenAPI security scheme keys. A rule matches when the middleware runtime
// function name contains RuleContains.
type MiddlewareSecurityRule struct {
	RuleContains string
	Schemes      []string
}

// RouteSecurityRule maps a method+route suffix pattern to one or more
// OpenAPI security scheme keys.
type RouteSecurityRule struct {
	Method     string
	PathSuffix string
	Schemes    []string
}

// SecurityInferenceConfig controls how annot8 infers operation security.
//
// 1) MiddlewareRules are the primary source (1:1 explicit mapping).
// 2) RouteRules are documented exceptions where auth is handler-driven.
// 3) ACL fallback is optional and can be disabled.
type SecurityInferenceConfig struct {
	MiddlewareRules                []MiddlewareSecurityRule
	RouteRules                     []RouteSecurityRule
	EnableACLBearerFromPermissions bool
}

// DefaultSecurityInferenceConfig returns annot8's default security mapping.
//
// It includes the legacy JWT/auth name-contains behavior as explicit rules,
// so behavior is auditable and overrideable.
func DefaultSecurityInferenceConfig() SecurityInferenceConfig {
	return SecurityInferenceConfig{
		MiddlewareRules: []MiddlewareSecurityRule{
			// {RuleContains: "Authenticated", Schemes: []string{"BearerAuth"}},
		},
		RouteRules: []RouteSecurityRule{
			// {
			// 	Method:     http.MethodPost,
			// 	PathSuffix: "/refresh",
			// 	Schemes:    []string{"RefreshTokenCookieAuth"},
			// },
		},
		EnableACLBearerFromPermissions: true,
	}
}

func inferOperationSecurity(
	route, method string,
	middlewares []func(http.Handler) http.Handler,
	cfg SecurityInferenceConfig,
) []SecurityRequirement {
	required := make(SecurityRequirement)

	for _, mw := range middlewares {
		name := middlewareRuntimeName(mw)
		if name == "" {
			continue
		}

		for _, rule := range cfg.MiddlewareRules {
			if strings.TrimSpace(rule.RuleContains) == "" {
				continue
			}
			if !strings.Contains(name, rule.RuleContains) {
				continue
			}
			for _, scheme := range rule.Schemes {
				required[scheme] = []string{}
			}
		}
	}

	for _, rule := range cfg.RouteRules {
		if rule.Method != "" && !strings.EqualFold(rule.Method, method) {
			continue
		}
		if rule.PathSuffix != "" && !strings.HasSuffix(strings.TrimSpace(route), rule.PathSuffix) {
			continue
		}
		for _, scheme := range rule.Schemes {
			required[scheme] = []string{}
		}
	}

	if len(required) == 0 {
		return nil
	}
	// One requirement object means all listed schemes are required together (AND semantics).
	return []SecurityRequirement{required}
}

func middlewareRuntimeName(mw func(http.Handler) http.Handler) string {
	if mw == nil {
		return ""
	}
	if fn := runtime.FuncForPC(reflect.ValueOf(mw).Pointer()); fn != nil {
		return fn.Name()
	}
	return ""
}
