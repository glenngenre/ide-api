package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"skwtr-ide-backend/db"
	"skwtr-ide-backend/middleware"
)

type loginRequest struct {
	Username string `json:"username" example:"admin"`
	Password string `json:"password" example:"supersecret"`
}

type registerRequest struct {
	Username string `json:"username" example:"glenn"`
	Password string `json:"password" example:"supersecret"`
	Role     string `json:"role"     example:"user" enums:"admin,user"`
}

type authResponse struct {
	Token        string `json:"token"         example:"eyJhbGci..."`
	RefreshToken string `json:"refresh_token" example:"7Yq2..."`
	Username     string `json:"username"      example:"admin"`
	Role         string `json:"role"          example:"admin"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"7Yq2..."`
}

type tokenPair struct {
	Token        string `json:"token"         example:"eyJhbGci..."`
	RefreshToken string `json:"refresh_token" example:"7Yq2..."`
	Username     string `json:"username"      example:"admin"`
	Role         string `json:"role"          example:"admin"`
}

// Login godoc
//
//	@Summary		Login
//	@Description	Authenticate with username and password, receive a JWT.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginRequest	true	"Credentials"
//	@Success		200		{object}	authResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/v1/auth/login [post]
func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := db.GetUserByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	access, err := middleware.IssueToken(user.ID, user.Username, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	refreshPlain, refreshHash, err := middleware.NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	familyID, err := middleware.NewFamilyID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	if err := db.CreateRefreshToken(
		user.ID, refreshHash, familyID, time.Now().Add(middleware.RefreshTTL()),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token:        access,
		RefreshToken: refreshPlain,
		Username:     user.Username,
		Role:         user.Role,
	})
}

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Create a new user. Requires an admin JWT.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerRequest	true	"New user details"
//	@Success		201		{object}	models.User
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		403		{object}	errorResponse
//	@Failure		409		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/auth/register [post]
func Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	role := req.Role
	if role != "admin" && role != "user" {
		role = "user"
	}

	existing, err := db.GetUserByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "username already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := db.CreateUser(req.Username, string(hash), role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// ListUsers godoc
//
//	@Summary		List all users
//	@Description	Returns all registered users. Requires an admin JWT.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{array}		models.User
//	@Failure		401	{object}	errorResponse
//	@Failure		403	{object}	errorResponse
//	@Failure		500	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/auth/users [get]
func ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	users, err := db.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, users)
}

// DeleteUser godoc
//
//	@Summary		Delete a user
//	@Description	Delete a user by ID. Requires an admin JWT. Admins cannot delete themselves.
//	@Tags			auth
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		204	"No Content"
//	@Failure		400	{object}	errorResponse
//	@Failure		401	{object}	errorResponse
//	@Failure		403	{object}	errorResponse
//	@Failure		500	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/auth/users/{id} [delete]
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}

	idStr := parts[len(parts)-1]
	id, err := parseInt64(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	caller := middleware.GetClaims(r)
	if caller.UserID == id {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := db.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Logout godoc
//
//	@Summary		Logout a user
//	@Description	Revokes the submitted refresh token and its entire token family.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		refreshRequest	true	"Refresh token"
//	@Success		204	"No Content"
//	@Failure		400	{object}	errorResponse
//	@Failure		500	{object}	errorResponse
//	@Router			/v1/auth/logout [post]
func Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rt, err := db.GetRefreshToken(middleware.HashRefreshToken(req.RefreshToken))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if rt != nil {
		if err := db.RevokeFamily(rt.FamilyID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// Refresh godoc
//
//	@Summary		Exchange a refresh token for a new token pair
//	@Description	Rotates the refresh token. Reusing a revoked token revokes the entire token family.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		refreshRequest	true	"Refresh token"
//	@Success		200		{object}	authResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/v1/auth/refresh [post]
func Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	rt, err := db.GetRefreshToken(middleware.HashRefreshToken(req.RefreshToken))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if rt == nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	if rt.RevokedAt != nil {
		_ = db.RevokeFamily(rt.FamilyID)
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if time.Now().After(rt.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	ok, err := db.RevokeRefreshTokenIfActive(rt.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := db.GetUserByID(rt.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	access, err := middleware.IssueToken(user.ID, user.Username, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	refreshPlain, refreshHash, err := middleware.NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	if err := db.CreateRefreshToken(
		user.ID, refreshHash, rt.FamilyID, time.Now().Add(middleware.RefreshTTL()),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token:        access,
		RefreshToken: refreshPlain,
		Username:     user.Username,
		Role:         user.Role,
	})
}
