package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"noted/cmd/server/handler"
)

// OriginCheck is the CSRF defense for state-changing verbs: a browser
// always attaches Origin (or, older, Referer) to cross-site POSTs, so
// the header must match the configured APP_ORIGIN or the request's own
// host. Absent both headers means a non-browser client — SameSite=Lax
// plus the browser's own Origin requirement cover the CSRF surface
// (SECURITY.md "Cookies / CSRF / CORS"). API.md freezes which verbs are
// mutations: POST/PATCH/DELETE.
func OriginCheck(appOrigin string) func(http.Handler) http.Handler {
	allow := strings.TrimSuffix(appOrigin, "/")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			originCheck(w, r, next, allow)
		})
	}
}

// originCheck is OriginCheck's handler body: reject a cross-site
// mutation, otherwise pass the request on.
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

// requestOriginOK trusts the Origin header when present, else Referer;
// with neither, the request did not come from a browser form/fetch.
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

// originAllowed accepts the exact APP_ORIGIN or any origin on the
// request's own host (scheme-agnostic: the browser's Origin carries
// http/https, the Host header does not, and both identify the site).
func originAllowed(origin, host, allow string) bool {
	if origin == allow {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == host && u.Host != ""
}
