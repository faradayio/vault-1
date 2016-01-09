package http

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/vault"
)

func handleSysRootGenerationInit(core *vault.Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleSysRootGenerationInitGet(core, w, r)
		case "POST", "PUT":
			handleSysRootGenerationInitPut(core, w, r)
		case "DELETE":
			handleSysRootGenerationInitDelete(core, w, r)
		default:
			respondError(w, http.StatusMethodNotAllowed, nil)
		}
	})
}

func handleSysRootGenerationInitGet(core *vault.Core, w http.ResponseWriter, r *http.Request) {
	// Get the current seal configuration
	sealConfig, err := core.SealConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	if sealConfig == nil {
		respondError(w, http.StatusBadRequest, fmt.Errorf(
			"server is not yet initialized"))
		return
	}

	// Get the generation configuration
	generationConfig, err := core.RootGenerationConfiguration()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	// Get the progress
	progress, err := core.RootGenerationProgress()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	// Format the status
	status := &RootGenerationStatusResponse{
		Started:  false,
		Progress: progress,
		Required: sealConfig.SecretThreshold,
		Complete: false,
	}
	if generationConfig != nil {
		status.Nonce = generationConfig.Nonce
		status.Started = true
	}

	respondOk(w, status)
}

func handleSysRootGenerationInitPut(core *vault.Core, w http.ResponseWriter, r *http.Request) {
	req := requestAuth(r, &logical.Request{})
	// Initialize the generation
	err := core.RootGenerationInit(req.ClientToken)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondOk(w, nil)
}

func handleSysRootGenerationInitDelete(core *vault.Core, w http.ResponseWriter, r *http.Request) {
	err := core.RootGenerationCancel()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	respondOk(w, nil)
}

func handleSysRootGenerationUpdate(core *vault.Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			respondError(w, http.StatusMethodNotAllowed, nil)
			return
		}

		// Parse the request
		var req RootGenerationUpdateRequest
		if err := parseRequest(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		if req.Key == "" {
			respondError(
				w, http.StatusBadRequest,
				errors.New("'key' must specified in request body as JSON"))
			return
		}

		// Decode the key, which is hex encoded
		key, err := hex.DecodeString(req.Key)
		if err != nil {
			respondError(
				w, http.StatusBadRequest,
				errors.New("'key' must be a valid hex-string"))
			return
		}

		// Use the key to make progress on root generation
		result, err := core.RootGenerationUpdate(key, req.Nonce)
		if err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}

		resp := &RootGenerationStatusResponse{
			Complete: result.Progress == result.Required,
			Nonce:    req.Nonce,
			Progress: result.Progress,
			Required: result.Required,
			Started:  true,
		}

		respondOk(w, resp)
	})
}

type RootGenerationStatusResponse struct {
	Nonce    string `json:"nonce"`
	Started  bool   `json:"started"`
	Progress int    `json:"progress"`
	Required int    `json:"required"`
	Complete bool   `json:"complete"`
}

type RootGenerationUpdateRequest struct {
	Nonce string
	Key   string
}
