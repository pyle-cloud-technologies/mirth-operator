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

const stateStarted = "STARTED"

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
	if status.ServerStatusString() != stateStarted {
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
		resp := `{"list":{"dashboardStatus":[{"channelId":"ch-1","name":"HL7 Inbound","state":"STARTED","statistics":{"@class":"linked-hash-map","entry":[{"com.mirth.connect.donkey.model.message.Status":"RECEIVED","long":100},{"com.mirth.connect.donkey.model.message.Status":"SENT","long":95},{"com.mirth.connect.donkey.model.message.Status":"ERROR","long":5},{"com.mirth.connect.donkey.model.message.Status":"QUEUED","long":2}]},"childStatuses":{"dashboardStatus":[{"channelId":"ch-1","name":"Source","metaDataId":0,"state":"STARTED","statistics":{"@class":"linked-hash-map","entry":[{"com.mirth.connect.donkey.model.message.Status":"RECEIVED","long":100}]}},{"channelId":"ch-1","name":"dest-1","metaDataId":1,"state":"STARTED","statistics":{"@class":"linked-hash-map","entry":[{"com.mirth.connect.donkey.model.message.Status":"RECEIVED","long":100},{"com.mirth.connect.donkey.model.message.Status":"SENT","long":90},{"com.mirth.connect.donkey.model.message.Status":"FILTERED","long":10}]}}]}},{"channelId":"ch-2","name":"FHIR Outbound","state":"STOPPED"}]}}`
		_, _ = w.Write([]byte(resp))
	})

	client := newTestServer(t, mux)

	statuses, err := client.GetChannelStatuses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].State != stateStarted {
		t.Errorf("expected STARTED, got %s", statuses[0].State)
	}
	if statuses[1].State != "STOPPED" {
		t.Errorf("expected STOPPED, got %s", statuses[1].State)
	}
	stats := statuses[0].ParseStatistics()
	if stats.Received != 100 {
		t.Errorf("expected 100 received, got %d", stats.Received)
	}

	// Verify childStatuses were parsed
	children := statuses[0].ParseChildStatuses()
	if len(children) != 2 {
		t.Fatalf("expected 2 child statuses, got %d", len(children))
	}
	if children[0].Name != "Source" || children[0].MetaDataID != 0 {
		t.Errorf("expected Source with metaDataId 0, got %s/%d", children[0].Name, children[0].MetaDataID)
	}
	if children[1].Name != "dest-1" || children[1].MetaDataID != 1 {
		t.Errorf("expected dest-1 with metaDataId 1, got %s/%d", children[1].Name, children[1].MetaDataID)
	}
	destStats := children[1].ParseStatistics()
	if destStats.Sent != 90 {
		t.Errorf("expected dest-1 sent=90, got %d", destStats.Sent)
	}
	if destStats.Filtered != 10 {
		t.Errorf("expected dest-1 filtered=10, got %d", destStats.Filtered)
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

func TestGetEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("minEventId"); got != "42" {
			t.Errorf("expected minEventId=42, got %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("expected limit=50, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := `{"list":{"event":[
			{"id":43,"level":"ERROR","name":"Channel Deployed","outcome":"FAILURE","userId":1,"dateTime":"2026-04-21T10:00:00Z","attributes":{"channelId":"evt-ch","channelName":"Events Channel"}},
			{"id":44,"level":"INFO","name":"Server Startup","outcome":"SUCCESS","userId":1,"dateTime":"2026-04-21T10:01:00Z","attributes":{}}
		]}}`
		_, _ = w.Write([]byte(resp))
	})

	client := newTestServer(t, mux)

	events, err := client.GetEvents(context.Background(), 42, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if !events[0].IsDeployError() {
		t.Error("expected first event (Channel Deployed / ERROR) to be flagged as deploy error")
	}
	if events[1].IsDeployError() {
		t.Error("expected second event (Server Startup / INFO) not to be flagged")
	}

	id, name := events[0].ChannelRef()
	if id != "evt-ch" || name != "Events Channel" {
		t.Errorf("expected channel ref evt-ch/Events Channel, got %s/%s", id, name)
	}
}

func TestGetEventsServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	client := newTestServer(t, mux)

	_, err := client.GetEvents(context.Background(), 0, 10)
	if err == nil {
		t.Fatal("expected error for 500 response")
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

func TestParseChildStatuses(t *testing.T) {
	t.Run("array form with source and destination", func(t *testing.T) {
		ds := DashboardStatus{
			ChannelID: "ch-1",
			Name:      "Test Channel",
			ChildStatuses: json.RawMessage(`{"dashboardStatus":[
				{"channelId":"ch-1","name":"Source","metaDataId":0,"state":"STARTED","statistics":{"@class":"linked-hash-map","entry":[{"com.mirth.connect.donkey.model.message.Status":"RECEIVED","long":50}]}},
				{"channelId":"ch-1","name":"better_rx","metaDataId":1,"state":"STARTED","statistics":{"@class":"linked-hash-map","entry":[{"com.mirth.connect.donkey.model.message.Status":"RECEIVED","long":50},{"com.mirth.connect.donkey.model.message.Status":"SENT","long":0},{"com.mirth.connect.donkey.model.message.Status":"FILTERED","long":50}]}}
			]}`),
		}

		children := ds.ParseChildStatuses()
		if len(children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(children))
		}
		if children[0].MetaDataID != 0 {
			t.Errorf("expected source metaDataId=0, got %d", children[0].MetaDataID)
		}
		if children[1].Name != "better_rx" || children[1].MetaDataID != 1 {
			t.Errorf("expected better_rx/1, got %s/%d", children[1].Name, children[1].MetaDataID)
		}

		destStats := children[1].ParseStatistics()
		if destStats.Received != 50 {
			t.Errorf("expected received=50, got %d", destStats.Received)
		}
		if destStats.Sent != 0 {
			t.Errorf("expected sent=0, got %d", destStats.Sent)
		}
		if destStats.Filtered != 50 {
			t.Errorf("expected filtered=50, got %d", destStats.Filtered)
		}
	})

	t.Run("single element form", func(t *testing.T) {
		ds := DashboardStatus{
			ChildStatuses: json.RawMessage(`{"dashboardStatus":{"channelId":"ch-2","name":"solo_dest","metaDataId":1,"state":"STARTED"}}`),
		}

		children := ds.ParseChildStatuses()
		if len(children) != 1 {
			t.Fatalf("expected 1 child, got %d", len(children))
		}
		if children[0].Name != "solo_dest" {
			t.Errorf("expected solo_dest, got %s", children[0].Name)
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		ds := DashboardStatus{}
		if children := ds.ParseChildStatuses(); children != nil {
			t.Errorf("expected nil, got %v", children)
		}
	})
}
