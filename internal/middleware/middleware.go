// Package middleware
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/tokens"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

type UserMiddleware struct {
	UserStore store.UserStore
}

type contextKey string

const UserContextKey = contextKey("user")

// SessionCookieName is the cookie the browser UI uses to carry the bearer token.
const SessionCookieName = "session"

func SetUser(r *http.Request, user *store.User) *http.Request {
	ctx := context.WithValue(r.Context(), UserContextKey, user)
	return r.WithContext(ctx)
}

func GetUser(r *http.Request) *store.User {
	user, ok := r.Context().Value(UserContextKey).(*store.User)
	if !ok {
		panic("missing user in request")
	}

	return user
}

func (um *UserMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")

		// The bearer header wins (programmatic clients); the browser UI falls back
		// to the session cookie carrying the same token.
		var token string
		fromCookie := false
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			headerParts := strings.Split(authHeader, " ") // Bearer <TOKEN>
			if len(headerParts) != 2 || headerParts[0] != "Bearer" {
				utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "invalid authorization header"})
				return
			}
			token = headerParts[1]
		} else if c, err := r.Cookie(SessionCookieName); err == nil {
			token = c.Value
			fromCookie = true
		}

		if token == "" {
			r = SetUser(r, store.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		user, err := um.UserStore.GetUserToken(tokens.ScopeAuth, token)
		if err != nil {
			utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "invalid token"})
			return
		}

		// A stale/invalid cookie should not hard-fail the browser: treat it as
		// anonymous so RequireUserWeb can redirect to the login page.
		if user == nil {
			if fromCookie {
				r = SetUser(r, store.AnonymousUser)
				next.ServeHTTP(w, r)
				return
			}
			utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "token expired or invalid"})
			return
		}

		r = SetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

func (um *UserMiddleware) RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)

		if user.IsAnonymous() {
			utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "you must be logged in to access this route"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireUserWeb mirrors RequireUser for browser routes: anonymous visitors are
// redirected to the login page instead of receiving a JSON 401.
func (um *UserMiddleware) RequireUserWeb(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetUser(r).IsAnonymous() {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}
