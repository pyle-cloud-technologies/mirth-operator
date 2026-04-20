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

import "encoding/json"

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
	FreeMemory    int64                  `json:"freeMemoryBytes"`
	AllocMemory   int64                  `json:"allocatedMemoryBytes"`
	MaxMemory     int64                  `json:"maxMemoryBytes"`
	CPUUsagePct   float64                `json:"cpuUsagePct"`
	DiskFreeBytes int64                  `json:"diskFreeBytes"`
	DiskTotal     int64                  `json:"diskTotalBytes"`
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
type DashboardStatus struct {
	ChannelID     string          `json:"channelId"`
	Name          string          `json:"name"`
	State         string          `json:"state"` // STARTED, STOPPED, PAUSED, ERROR
	DeployedDate  *MirthTimestamp `json:"deployedDate,omitempty"`
	Statistics    json.RawMessage `json:"statistics,omitempty"`
	ChildStatuses json.RawMessage `json:"childStatuses,omitempty"`
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
