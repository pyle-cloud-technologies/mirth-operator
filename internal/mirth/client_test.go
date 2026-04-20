/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mirth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.Handler) Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return NewClient(ClientConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: "admin",
	})
}

func TestGetServerStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/server/status", func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing Accept header")
		}
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Error("missing X-Requested-With header")
		}

		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "admin" {
			t.Error("invalid basic auth")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ServerStatusResponse{Int: 0})
	})

	client := newTestServer(t, mux)

	status, err := client.GetServerStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.ServerStatusString() != "STARTED" {
		t.Errorf("expected STARTED, got %s", status.ServerStatusString())
	}
}

func TestGetSystemStats(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/system/stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SystemStatsResponse{
			Stats: SystemStats{
				FreeMemory:  1024,
				AllocMemory: 4096,
				MaxMemory:   8192,
			},
		})
	})

	client := newTestServer(t, mux)

	stats, err := client.GetSystemStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.MaxMemory != 8192 {
		t.Errorf("expected MaxMemory 8192, got %d", stats.MaxMemory)
	}
}

func TestGetChannelStatuses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/channels/statuses", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DashboardStatusListResponse{
			List: &DashboardStatusList{
				DashboardStatuses: []DashboardStatus{
					{
						ChannelID: "ch-1",
						Name:      "HL7 Inbound",
						State:     "STARTED",
						Statistics: &ChannelStatistics{
							Received: 100,
							Sent:     95,
							Error:    5,
							Filtered: 0,
							Queued:   2,
						},
					},
					{
						ChannelID: "ch-2",
						Name:      "FHIR Outbound",
						State:     "STOPPED",
					},
				},
			},
		})
	})

	client := newTestServer(t, mux)

	statuses, err := client.GetChannelStatuses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].State != "STARTED" {
		t.Errorf("expected STARTED, got %s", statuses[0].State)
	}
	if statuses[1].State != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", statuses[1].State)
	}
	if statuses[0].Statistics.Received != 100 {
		t.Errorf("expected 100 received, got %d", statuses[0].Statistics.Received)
	}
}

func TestGetChannelStatistics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/channels/{id}/statistics", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id != "ch-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChannelStatistics{
			ChannelID: "ch-1",
			Received:  500,
			Sent:      490,
			Error:     10,
			Filtered:  0,
			Queued:    3,
		})
	})

	client := newTestServer(t, mux)

	stats, err := client.GetChannelStatistics(context.Background(), "ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Received != 500 {
		t.Errorf("expected 500 received, got %d", stats.Received)
	}
}

func TestRestartChannel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/channels/{id}/_restart", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id != "ch-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client := newTestServer(t, mux)

	err := client.RestartChannel(context.Background(), "ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartChannel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/channels/{id}/_start", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id != "ch-2" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client := newTestServer(t, mux)

	err := client.StartChannel(context.Background(), "ch-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/server/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})

	client := newTestServer(t, mux)

	_, err := client.GetServerStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestUnauthorizedResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/server/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	})

	client := newTestServer(t, mux)

	_, err := client.GetServerStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}
