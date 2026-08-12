package annot8fixtures_test

import "net/http"

func RequireTerminal() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func IsTenant(next http.Handler) http.Handler { return next }

func RequireDualIdentity(next http.Handler) http.Handler { return next }
