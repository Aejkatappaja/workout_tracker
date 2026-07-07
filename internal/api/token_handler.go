package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/tokens"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

type TokenHandler struct {
	tokenStore store.TokenStore
	userStore  store.UserStore
}

type createTokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewTokenHandler(tokenStore store.TokenStore, userStore store.UserStore) *TokenHandler {
	return &TokenHandler{
		tokenStore: tokenStore,
		userStore:  userStore,
	}
}

func (h *TokenHandler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		clientError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	user, err := h.userStore.GetUserByUsername(req.Username)
	if err != nil {
		serverError(w, r, "get user by username", err)
		return
	}

	if user == nil {
		store.FakePasswordCompare() // keep timing constant for unknown usernames
		clientError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	passwordsDoMatch, err := user.PasswordHash.Matches(req.Password)
	if err != nil {
		serverError(w, r, "compare password", err)
		return
	}

	if !passwordsDoMatch {
		clientError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.tokenStore.CreateNewToken(user.ID, 24*time.Hour, tokens.ScopeAuth)
	if err != nil {
		serverError(w, r, "create token", err)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, utils.Envelope{"auth_token": token})
}
