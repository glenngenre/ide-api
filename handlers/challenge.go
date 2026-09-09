package handlers

import (
	"net/http"
	"strings"
	"time"

	"skwtr-ide-backend/db"
	"skwtr-ide-backend/middleware"
)

// DailyChallenges godoc
//
//	@Summary		Get today's coding challenge
//	@Description	Returns today's challenge and whether the authenticated user has completed it.
//	@Tags			challenges
//	@Produce		json
//	@Success		200		{array}		models.Challenge
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/challenges/daily [get]
func DailyChallenges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims := middleware.GetClaims(r)
	challenges, err := db.GetDailyChallenges(time.Now().UTC().Format("2006-01-02"), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load daily challenge")
		return
	}
	if len(challenges) == 0 {
		writeError(w, http.StatusNotFound, "no daily challenge found")
		return
	}

	writeJSON(w, http.StatusOK, challenges)
}

// CompleteChallenge godoc
//
//	@Summary		Mark a challenge as solved
//	@Tags			challenges
//	@Produce		json
//	@Param			id	path		int	true	"Challenge ID"
//	@Success		204	"No Content"
//	@Failure		400	{object}	errorResponse
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		500	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/challenges/{id}/complete [post]
func CompleteChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, http.StatusBadRequest, "invalid challenge id")
		return
	}
	id, err := parseInt64(parts[len(parts)-2])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid challenge id")
		return
	}

	if err := db.CompleteChallenge(id, middleware.GetClaims(r).UserID); err != nil {
		if err == db.ErrChallengeNotFound {
			writeError(w, http.StatusNotFound, "challenge not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to complete challenge")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
