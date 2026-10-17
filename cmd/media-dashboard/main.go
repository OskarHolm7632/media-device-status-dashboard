package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	dashboard "example.com/media-device-dashboard"
)

const channel = "media-device-status"

type server struct {
	client   *dashboard.Client
	workflow dashboard.Workflow
}

func main() {
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = dashboard.DefaultBaseURL
	}
	client, err := dashboard.NewClient(baseURL, os.Getenv("INFRAI_API_KEY"), nil)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := contextWithDeadline()
	defer cancel()
	if err := client.CreateChannel(ctx, channel); err != nil {
		var apiErr *dashboard.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
			log.Fatal(err)
		}
	}

	s := server{client: client, workflow: dashboard.Workflow{Channel: channel, Publisher: client}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /device-updates", s.recordUpdate)
	mux.HandleFunc("POST /dashboard-session", s.dashboardSession)

	log.Printf("media dashboard listening on %s", ":8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func contextWithDeadline() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func (s server) recordUpdate(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var update dashboard.DeviceUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	snapshot, err := s.workflow.Record(r.Context(), update)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, snapshot)
}

func (s server) dashboardSession(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client_id is required"})
		return
	}
	token, err := s.client.IssueToken(r.Context(), input.ClientID, []string{channel})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	metrics, err := s.client.QueryMetrics(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channel": channel, "realtime": token, "metrics": metrics})
}

func writeServiceError(w http.ResponseWriter, err error) {
	var apiErr *dashboard.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]any{"error": apiErr.Detail})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
