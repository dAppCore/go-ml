package ml

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// OllamaBaseModelMap maps model tags to Ollama model names.
var OllamaBaseModelMap = map[string]string{
	"gemma-3-1b":  "gemma3:1b",
	"gemma-3-4b":  "gemma3:4b",
	"gemma-3-12b": "gemma3:12b",
	"gemma-3-27b": "gemma3:27b",
}

// HFBaseModelMap maps model tags to HuggingFace model IDs.
var HFBaseModelMap = map[string]string{
	"gemma-3-1b":  "google/gemma-3-1b-it",
	"gemma-3-4b":  "google/gemma-3-4b-it",
	"gemma-3-12b": "google/gemma-3-12b-it",
	"gemma-3-27b": "google/gemma-3-27b-it",
}

// ollamaUploadBlob uploads a local file to Ollama's blob store.
// Returns the sha256 digest string (e.g. "sha256:abc123...").
func ollamaUploadBlob(ollamaURL, filePath string) (string, error) {
	raw, err := coreio.Local.Read(filePath)
	if err != nil {
		return "", coreerr.E("ml.ollamaUploadBlob", core.Sprintf("read %s", filePath), err)
	}
	data := []byte(raw)

	hash := sha256.Sum256(data)
	digest := "sha256:" + hex.EncodeToString(hash[:])

	headReq, _ := http.NewRequest(http.MethodHead, ollamaURL+"/api/blobs/"+digest, nil)
	client := &http.Client{Timeout: 5 * time.Minute}
	headResp, err := client.Do(headReq)
	if err == nil && headResp.StatusCode == http.StatusOK {
		headResp.Body.Close()
		return digest, nil
	}
	if headResp != nil {
		headResp.Body.Close()
	}

	req, err := http.NewRequest(http.MethodPost, ollamaURL+"/api/blobs/"+digest, bytes.NewReader(data))
	if err != nil {
		return "", coreerr.E("ml.ollamaUploadBlob", "blob request", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return "", coreerr.E("ml.ollamaUploadBlob", "blob upload", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", coreerr.E("ml.ollamaUploadBlob", core.Sprintf("blob upload HTTP %d: %s", resp.StatusCode, string(body)), nil)
	}
	return digest, nil
}

// OllamaCreateModel creates a temporary Ollama model with a LoRA adapter.
// peftDir is a local directory containing adapter_model.safetensors and adapter_config.json.
func OllamaCreateModel(ollamaURL, modelName, baseModel, peftDir string) error {
	sfPath := peftDir + "/adapter_model.safetensors"
	cfgPath := peftDir + "/adapter_config.json"

	sfDigest, err := ollamaUploadBlob(ollamaURL, sfPath)
	if err != nil {
		return coreerr.E("ml.OllamaCreateModel", "upload adapter safetensors", err)
	}

	cfgDigest, err := ollamaUploadBlob(ollamaURL, cfgPath)
	if err != nil {
		return coreerr.E("ml.OllamaCreateModel", "upload adapter config", err)
	}

	reqBody := core.JSONMarshalString(map[string]any{
		"model": modelName,
		"from":  baseModel,
		"adapters": map[string]string{
			"adapter_model.safetensors": sfDigest,
			"adapter_config.json":       cfgDigest,
		},
	})

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Post(ollamaURL+"/api/create", "application/json", bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return coreerr.E("ml.OllamaCreateModel", "ollama create", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var status struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if r := core.JSONUnmarshalString(scanner.Text(), &status); !r.OK {
			return coreerr.E("ml.OllamaCreateModel", "ollama create decode", r.Value.(error))
		}
		if status.Error != "" {
			return coreerr.E("ml.OllamaCreateModel", core.Sprintf("ollama create: %s", status.Error), nil)
		}
		if status.Status == "success" {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return coreerr.E("ml.OllamaCreateModel", "ollama create decode", err)
	}

	if resp.StatusCode != http.StatusOK {
		return coreerr.E("ml.OllamaCreateModel", core.Sprintf("ollama create: HTTP %d", resp.StatusCode), nil)
	}
	return nil
}

// OllamaDeleteModel removes a temporary Ollama model.
func OllamaDeleteModel(ollamaURL, modelName string) error {
	body := core.JSONMarshalString(map[string]string{"model": modelName})

	req, err := http.NewRequest(http.MethodDelete, ollamaURL+"/api/delete", bytes.NewReader([]byte(body)))
	if err != nil {
		return coreerr.E("ml.OllamaDeleteModel", "ollama delete request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return coreerr.E("ml.OllamaDeleteModel", "ollama delete", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return coreerr.E("ml.OllamaDeleteModel", core.Sprintf("ollama delete %d: %s", resp.StatusCode, string(respBody)), nil)
	}
	return nil
}
