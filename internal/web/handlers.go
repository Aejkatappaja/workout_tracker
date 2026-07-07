package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/tokens"
	"github.com/Aejkatappaja/go-gym/internal/web/views"
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

// baseURL reconstructs the site's absolute origin from the request, so email
// links point back at whatever host (custom domain or code.run) served the page.
func baseURL(r *http.Request) string {
	scheme := "http"
	if secureRequest(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// sendMail renders the HTML body once, then delivers in the background so the
// HTTP response is never blocked on the mail provider. Failures are logged.
func (h *Handler) sendMail(log *slog.Logger, to, subject string, body templ.Component, text string) {
	var sb strings.Builder
	if err := body.Render(context.Background(), &sb); err != nil {
		log.Error("render email", "to", to, "err", err)
		return
	}
	html := sb.String()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := h.mailer.Send(ctx, to, subject, html, text); err != nil {
			log.Error("send email", "to", to, "err", err)
		}
	}()
}

func welcomeText(username, appURL string) string {
	return "welcome, " + username + "\n\n" +
		"Your go-gym account is ready. Log your workouts, track reps or duration, and watch your activity heatmap fill in.\n\n" +
		"Open go-gym: " + appURL + "\n\ngo-gym"
}

func resetText(resetURL string) string {
	return "reset your go-gym password\n\n" +
		"Use this link to choose a new password (expires in 1 hour, single use):\n" + resetURL + "\n\n" +
		"If you didn't request this, you can ignore this email. Your password won't change.\n\ngo-gym"
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

// DemoLogin drops the visitor into the read-only demo account without a password.
func (h *Handler) DemoLogin(w http.ResponseWriter, r *http.Request) {
	demo, err := h.users.GetUserByUsername(store.DemoUsername)
	if err != nil || demo == nil {
		middleware.LoggerFrom(r.Context()).Error("demo login lookup", "err", err)
		h.render(w, r, http.StatusServiceUnavailable, views.LoginPage("the demo is unavailable right now"))
		return
	}
	if err := h.setSession(w, r, demo.ID); err != nil {
		middleware.LoggerFrom(r.Context()).Error("demo session", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.LoginPage("something went wrong, try again"))
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.GetUserByUsername(r.FormValue("username"))
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("web login lookup", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.LoginPage("something went wrong, try again"))
		return
	}

	if user == nil {
		store.FakePasswordCompare() // keep timing constant for unknown usernames
		h.render(w, r, http.StatusUnauthorized, views.LoginPage("invalid credentials"))
		return
	}

	ok, err := user.PasswordHash.Matches(r.FormValue("password"))
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("web login hash", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.LoginPage("something went wrong, try again"))
		return
	}
	if !ok {
		h.render(w, r, http.StatusUnauthorized, views.LoginPage("invalid credentials"))
		return
	}

	if err := h.setSession(w, r, user.ID); err != nil {
		middleware.LoggerFrom(r.Context()).Error("web login session", "err", err)
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
		middleware.LoggerFrom(r.Context()).Error("web register hash", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.RegisterPage("something went wrong, try again"))
		return
	}

	if err := h.users.CreateUser(user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			h.render(w, r, http.StatusConflict, views.RegisterPage("username or email already taken"))
			return
		}
		middleware.LoggerFrom(r.Context()).Error("web register create", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.RegisterPage("something went wrong, try again"))
		return
	}

	if err := h.setSession(w, r, user.ID); err != nil {
		middleware.LoggerFrom(r.Context()).Error("web register session", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.RegisterPage("something went wrong, try again"))
		return
	}

	appURL := baseURL(r) + "/app"
	h.sendMail(middleware.LoggerFrom(r.Context()), user.Email, "welcome to go-gym", views.WelcomeEmail(user.Username, appURL), welcomeText(user.Username, appURL))

	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

// ForgotForm renders the "enter your email" page.
func (h *Handler) ForgotForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, views.ForgotPage("", false))
}

// Forgot issues a password-reset link. The response is identical whether or not
// the email matches an account, so it never reveals which emails are registered.
func (h *Handler) Forgot(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		h.render(w, r, http.StatusBadRequest, views.ForgotPage("enter your email", false))
		return
	}

	user, err := h.users.GetUserByEmail(email)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("forgot lookup", "err", err)
	}
	// Skip the seeded demo account; otherwise send a 1-hour, single-use link.
	if user != nil && user.Username != store.DemoUsername {
		tok, terr := h.tokens.CreateNewToken(user.ID, time.Hour, tokens.ScopePasswordReset)
		if terr != nil {
			middleware.LoggerFrom(r.Context()).Error("forgot token", "err", terr)
		} else {
			resetURL := baseURL(r) + "/reset?token=" + url.QueryEscape(tok.PlainText)
			h.sendMail(middleware.LoggerFrom(r.Context()), user.Email, "reset your go-gym password", views.ResetEmail(resetURL), resetText(resetURL))
		}
	}

	h.render(w, r, http.StatusOK, views.ForgotPage("", true))
}

// ResetForm renders the "choose a new password" page for a reset link.
func (h *Handler) ResetForm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.render(w, r, http.StatusBadRequest, views.ResetPage("", "this reset link is invalid or has expired"))
		return
	}
	h.render(w, r, http.StatusOK, views.ResetPage(token, ""))
}

// Reset validates the token, sets the new password, and invalidates the reset
// token (single use) plus any existing sessions.
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")

	if token == "" {
		h.render(w, r, http.StatusBadRequest, views.ResetPage("", "this reset link is invalid or has expired"))
		return
	}
	if len(password) < 8 {
		h.render(w, r, http.StatusBadRequest, views.ResetPage(token, "password must be at least 8 characters"))
		return
	}

	user, err := h.users.GetUserToken(tokens.ScopePasswordReset, token)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("reset token lookup", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.ResetPage(token, "something went wrong, try again"))
		return
	}
	if user == nil {
		h.render(w, r, http.StatusBadRequest, views.ResetPage("", "this reset link is invalid or has expired"))
		return
	}

	if err := h.users.UpdateUserPassword(user.ID, password); err != nil {
		middleware.LoggerFrom(r.Context()).Error("reset update password", "err", err)
		h.render(w, r, http.StatusInternalServerError, views.ResetPage(token, "something went wrong, try again"))
		return
	}

	// single-use: drop the reset token(s), and revoke existing sessions so a
	// leaked cookie can't outlive the password change.
	if err := h.tokens.DeleteAllTokensForUser(user.ID, tokens.ScopePasswordReset); err != nil {
		middleware.LoggerFrom(r.Context()).Error("reset revoke reset tokens", "err", err)
	}
	if err := h.tokens.DeleteAllTokensForUser(user.ID, tokens.ScopeAuth); err != nil {
		middleware.LoggerFrom(r.Context()).Error("reset revoke sessions", "err", err)
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if user := middleware.GetUser(r); !user.IsAnonymous() {
		if err := h.tokens.DeleteAllTokensForUser(user.ID, tokens.ScopeAuth); err != nil {
			middleware.LoggerFrom(r.Context()).Error("web logout revoke", "err", err)
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
