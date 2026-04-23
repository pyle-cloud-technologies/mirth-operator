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
	"encoding/json"
	"strings"
)

// ServerStatusResponse represents the Mirth Connect server status response.
// GET /api/server/status returns {"int": N} where N is:
// 0=STARTED, 1=PAUSING, 2=PAUSED, 3=STOPPING, 4=STOPPED
type ServerStatusResponse struct {
	Int int `json:"int"`
}

// ServerStatusString converts the integer status to a human-readable string.
func (s ServerStatusResponse) ServerStatusString() string {
	switch s.Int {
	case 0:
		return "STARTED"
	case 1:
		return "PAUSING"
	case 2:
		return "PAUSED"
	case 3:
		return "STOPPING"
	case 4:
		return "STOPPED"
	default:
		return "UNKNOWN"
	}
}

// SystemStatsResponse wraps the Mirth system stats.
// GET /api/system/stats returns {"com.mirth.connect.model.SystemStats": {...}}
type SystemStatsResponse struct {
	Stats SystemStats `json:"com.mirth.connect.model.SystemStats"`
}

// SystemStats represents the Mirth Connect system statistics.
type SystemStats struct {
	Timestamp     map[string]any `json:"timestamp"`
	FreeMemory    int64          `json:"freeMemoryBytes"`
	AllocMemory   int64          `json:"allocatedMemoryBytes"`
	MaxMemory     int64          `json:"maxMemoryBytes"`
	CPUUsagePct   float64        `json:"cpuUsagePct"`
	DiskFreeBytes int64          `json:"diskFreeBytes"`
	DiskTotal     int64          `json:"diskTotalBytes"`
}

// DashboardStatusListResponse is the top-level response from GET /api/channels/statuses.
// Mirth wraps the list in {"list": {"dashboardStatus": [...]}}
type DashboardStatusListResponse struct {
	List *DashboardStatusList `json:"list"`
}

// DashboardStatusList wraps the Mirth dashboard status list response.
type DashboardStatusList struct {
	DashboardStatuses []DashboardStatus `json:"dashboardStatus"`
}

// MirthTimestamp represents Mirth's timestamp format: {"time": epoch_ms, "timezone": "..."}
type MirthTimestamp struct {
	Time     int64  `json:"time"`
	Timezone string `json:"timezone"`
}

// DashboardStatus represents a single channel's dashboard status from Mirth.
// Child entries (destinations) share the same shape; MetaDataID distinguishes
// source (0) from destination connectors (1+).
type DashboardStatus struct {
	ChannelID     string          `json:"channelId"`
	Name          string          `json:"name"`
	State         string          `json:"state"` // STARTED, STOPPED, PAUSED, ERROR
	MetaDataID    int             `json:"metaDataId,omitempty"`
	DeployedDate  *MirthTimestamp `json:"deployedDate,omitempty"`
	Statistics    json.RawMessage `json:"statistics,omitempty"`
	ChildStatuses json.RawMessage `json:"childStatuses,omitempty"`
	Queued        int64           `json:"queued,omitempty"`
}

// ConnectorStatus represents a connector (source or destination) status.
type ConnectorStatus struct {
	ChannelID  string             `json:"channelId"`
	Name       string             `json:"name"`
	State      string             `json:"state"`
	MetaDataID int                `json:"metaDataId"`
	Statistics *ChannelStatistics `json:"statistics,omitempty"`
	Queued     int64              `json:"queued"`
}

// ChannelStatistics represents parsed message statistics for a channel.
type ChannelStatistics struct {
	ChannelID string `json:"channelId,omitempty"`
	Received  int64  `json:"received"`
	Sent      int64  `json:"sent"`
	Error     int64  `json:"error"`
	Filtered  int64  `json:"filtered"`
	Queued    int64  `json:"queued"`
}

// mirthStatisticsRaw is the raw Mirth statistics format with entry arrays.
type mirthStatisticsRaw struct {
	Entries []mirthStatEntry `json:"entry"`
}

type mirthStatEntry struct {
	Status string `json:"com.mirth.connect.donkey.model.message.Status"`
	Value  int64  `json:"long"`
}

// ServerEventListResponse is the top-level envelope from GET /api/events.
// OIE wraps the list as {"list": {"event": [...]}}, matching the pattern used
// for channel statuses. Some builds may return the event(s) under a different
// top-level shape; ServerEventListResponse and ParseServerEvents together
// tolerate both the list-envelope and a bare array shape.
type ServerEventListResponse struct {
	List *ServerEventList `json:"list"`
}

// ServerEventList is the inner object wrapping the event slice.
type ServerEventList struct {
	Events []ServerEvent `json:"event"`
}

// ServerEvent is a single OIE server event. Fields mirror the shape returned
// by GET /api/events in OIE / Mirth Connect 4.x. Attributes is a free-form
// map populated with event-specific keys (commonly "channel", "channelId",
// "channelName").
type ServerEvent struct {
	ID         int64             `json:"id"`
	Level      string            `json:"level"`   // INFO / WARNING / ERROR
	Name       string            `json:"name"`    // e.g. "Channel Deployed"
	Outcome    string            `json:"outcome"` // SUCCESS / FAILURE
	UserID     int               `json:"userId"`
	DateTime   string            `json:"dateTime"`
	Attributes map[string]string `json:"attributes"`
}

// IsDeployError reports whether this event represents a channel deploy,
// script compile, or transformer/filter script failure. Matching is
// deliberately loose because OIE event naming varies — the upstream event ID
// guarantees we only count each event once even when the rule matches broadly.
func (e ServerEvent) IsDeployError() bool {
	level := strings.ToUpper(e.Level)
	outcome := strings.ToUpper(e.Outcome)
	name := strings.ToLower(e.Name)

	nameLooksDeployRelated := strings.Contains(name, "deploy") ||
		strings.Contains(name, "compile") ||
		strings.Contains(name, "script")

	if level == "ERROR" && nameLooksDeployRelated {
		return true
	}
	if outcome == "FAILURE" && nameLooksDeployRelated {
		return true
	}
	return false
}

// ChannelRef returns the channel id and name recorded on the event, if any.
// Keys vary across OIE builds, so multiple common attribute keys are tried.
// Either return value may be empty when the event is server-wide.
func (e ServerEvent) ChannelRef() (id, name string) {
	if e.Attributes == nil {
		return "", ""
	}
	for _, k := range []string{"channelId", "channel_id", "channelID"} {
		if v, ok := e.Attributes[k]; ok && v != "" {
			id = v
			break
		}
	}
	for _, k := range []string{"channelName", "channel_name", "channel"} {
		if v, ok := e.Attributes[k]; ok && v != "" {
			name = v
			break
		}
	}
	return id, name
}

// ParseServerEvents unmarshals an /api/events response, tolerating both the
// {"list":{"event":[...]}} envelope and a bare [...] array.
func ParseServerEvents(body []byte) ([]ServerEvent, error) {
	trimmed := strings.TrimLeft(string(body), " \t\r\n")
	if strings.HasPrefix(trimmed, "[") {
		var events []ServerEvent
		if err := json.Unmarshal(body, &events); err != nil {
			return nil, err
		}
		return events, nil
	}

	var resp ServerEventListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.List == nil {
		return nil, nil
	}
	return resp.List.Events, nil
}

// ParseChildStatuses extracts per-destination DashboardStatus entries from the
// raw childStatuses JSON. Mirth wraps these as {"dashboardStatus": [...]}.
// Handles the single-element quirk where Mirth serializes a lone child as
// {"dashboardStatus": {...}} instead of an array.
func (d *DashboardStatus) ParseChildStatuses() []DashboardStatus {
	if d.ChildStatuses == nil {
		return nil
	}

	var list DashboardStatusList
	if err := json.Unmarshal(d.ChildStatuses, &list); err == nil && len(list.DashboardStatuses) > 0 {
		return list.DashboardStatuses
	}

	var single struct {
		DashboardStatus DashboardStatus `json:"dashboardStatus"`
	}
	if err := json.Unmarshal(d.ChildStatuses, &single); err == nil && single.DashboardStatus.Name != "" {
		return []DashboardStatus{single.DashboardStatus}
	}

	return nil
}

// ParseStatistics extracts ChannelStatistics from the raw Mirth JSON format.
func (d *DashboardStatus) ParseStatistics() ChannelStatistics {
	stats := ChannelStatistics{ChannelID: d.ChannelID}
	if d.Statistics == nil {
		return stats
	}

	var raw mirthStatisticsRaw
	if err := json.Unmarshal(d.Statistics, &raw); err != nil {
		return stats
	}

	for _, e := range raw.Entries {
		switch e.Status {
		case "RECEIVED":
			stats.Received = e.Value
		case "SENT":
			stats.Sent = e.Value
		case "ERROR":
			stats.Error = e.Value
		case "FILTERED":
			stats.Filtered = e.Value
		case "QUEUED":
			stats.Queued = e.Value
		}
	}
	return stats
}
