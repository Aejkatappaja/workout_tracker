package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Aejkatappaja/workout_tracker/internal/middleware"
	"github.com/Aejkatappaja/workout_tracker/internal/store"
	"github.com/Aejkatappaja/workout_tracker/internal/tokens"
	"github.com/Aejkatappaja/workout_tracker/internal/web/views"
	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5/pgconn"
)

// secureRequest reports whether the request arrived over HTTPS, either directly
// or via a TLS-terminating proxy that sets X-Forwarded-Proto. Used to set the
// Secure flag on the session cookie so it is never sent over plain HTTP.
func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = c.Render(r.Context(), w)
}

// setSession issues a token and stores it in an HttpOnly session cookie.
func (h *Handler) setSession(w http.ResponseWriter, r *http.Request, userID int) error {
	tok, err := h.tokens.CreateNewToken(userID, 24*time.Hour, tokens.ScopeAuth)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    tok.PlainText,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureRequest(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  tok.Expiry,
	})
	return nil
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, views.LoginPage(""))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.GetUserByUsername(r.FormValue("username"))
	if err != nil {
		h.logger.Printf("ERROR: web login lookup: %v", err)
		h.render(w, r, http.StatusInternalServerError, views.LoginPage("something went wrong, try again"))
		return
	}

	if user == nil {
		h.render(w, r, http.StatusUnauthorized, views.LoginPage("invalid credentials"))
		return
	}

	ok, err := user.PasswordHash.Matches(r.FormValue("password"))
	if err != nil {
		h.logger.Printf("ERROR: web login hash: %v", err)
		h.render(w, r, http.StatusInternalServerError, views.LoginPage("something went wrong, try again"))
		return
	}
	if !ok {
		h.render(w, r, http.StatusUnauthorized, views.LoginPage("invalid credentials"))
		return
	}

	if err := h.setSession(w, r, user.ID); err != nil {
		h.logger.Printf("ERROR: web login session: %v", err)
		h.render(w, r, http.StatusInternalServerError, views.LoginPage("something went wrong, try again"))
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, views.RegisterPage(""))
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")

	if username == "" || email == "" || len(password) < 8 {
		h.render(w, r, http.StatusBadRequest, views.RegisterPage("username, email and an 8+ character password are required"))
		return
	}
	if len(username) > 50 {
		h.render(w, r, http.StatusBadRequest, views.RegisterPage("username cannot exceed 50 characters"))
		return
	}

	user := &store.User{Username: username, Email: email}
	if err := user.PasswordHash.Set(password); err != nil {
		h.logger.Printf("ERROR: web register hash: %v", err)
		h.render(w, r, http.StatusInternalServerError, views.RegisterPage("something went wrong, try again"))
		return
	}

	if err := h.users.CreateUser(user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			h.render(w, r, http.StatusConflict, views.RegisterPage("username or email already taken"))
			return
		}
		h.logger.Printf("ERROR: web register create: %v", err)
		h.render(w, r, http.StatusInternalServerError, views.RegisterPage("something went wrong, try again"))
		return
	}

	if err := h.setSession(w, r, user.ID); err != nil {
		h.logger.Printf("ERROR: web register session: %v", err)
		h.render(w, r, http.StatusInternalServerError, views.RegisterPage("something went wrong, try again"))
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if user := middleware.GetUser(r); !user.IsAnonymous() {
		if err := h.tokens.DeleteAllTokensForUser(user.ID, tokens.ScopeAuth); err != nil {
			h.logger.Printf("ERROR: web logout revoke: %v", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
