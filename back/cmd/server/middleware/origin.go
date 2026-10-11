package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"noted/cmd/server/handler"
)

// OriginCheck is the CSRF defense for state-changing verbs (POST/PATCH/
// DELETE per API.md): Origin — or, older, Referer — must match the
// configured APP_ORIGIN or the request's own host.
func OriginCheck(appOrigin string) Middleware {
	allow := strings.TrimSuffix(appOrigin, "/")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			originCheck(w, r, next, allow)
		})
	}
}

func originCheck(w http.ResponseWriter, r *http.Request, next http.Handler, allow string) {
	if isMutation(r.Method) && !requestOriginOK(r, allow) {
		handler.WriteError(w, http.StatusForbidden, "forbidden origin", nil)
		return
	}

	next.ServeHTTP(w, r)
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Neither header means a non-browser client: SameSite=Lax plus the
// browser's own Origin requirement cover that surface (SECURITY.md).
func requestOriginOK(r *http.Request, allow string) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return originAllowed(origin, r.Host, allow)
	}

	if referer := r.Header.Get("Referer"); referer != "" {
		u, err := url.Parse(referer)

		if err != nil {
			return false
		}

		return originAllowed(u.Scheme+"://"+u.Host, r.Host, allow)
	}
	return true
}

// originAllowed compares scheme-agnostic: the browser's Origin carries
// http/https, the Host header does not, and both identify the site.
func originAllowed(origin, host, allow string) bool {
	if origin == allow {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == host && u.Host != ""
}
