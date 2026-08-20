package internal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/javin1106/airport/internal/gitrepo"
)

type deployRequest struct {
	RepoURL string `json:"repoUrl"`
}

type deployResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func HandleDeploy(w http.ResponseWriter, r *http.Request) {
	var request deployRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "invalid request body",
		})
		return
	}

	request.RepoURL = strings.TrimSpace(request.RepoURL)

	if request.RepoURL == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "repoUrl is required",
		})
		return
	}

	deploymentID, err := generateDeploymentID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error: "failed to generate deployment ID",
		})
		return
	}

	cloneContext, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	destination := filepath.Join("output", deploymentID)
	if err := gitrepo.Clone(cloneContext, request.RepoURL, destination); err != nil {
		log.Printf("failed to clone the repository: %v", err)

		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
			Error: "failed to clone repository",
		})

		return
	}

	writeJSON(w, http.StatusAccepted, deployResponse{
		ID:     deploymentID,
		Status: "cloned",
	})
}

func generateDeploymentID() (string, error) {
	randomBytes := make([]byte, 8)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to encode json response: %v", err)
	}
}
