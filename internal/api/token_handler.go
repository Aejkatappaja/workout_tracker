package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
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
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("decode token request", "err", err)
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	user, err := h.userStore.GetUserByUsername(req.Username)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("get user by username", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if user == nil {
		store.FakePasswordCompare() // keep timing constant for unknown usernames
		utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "invalid credentials"})
		return
	}

	passwordsDoMatch, err := user.PasswordHash.Matches(req.Password)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("compare password", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if !passwordsDoMatch {
		utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "invalid credentials"})
		return
	}

	token, err := h.tokenStore.CreateNewToken(user.ID, 24*time.Hour, tokens.ScopeAuth)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("create token", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJSON(w, http.StatusCreated, utils.Envelope{"auth_token": token})
}
