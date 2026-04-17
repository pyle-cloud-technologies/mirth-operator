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

// ServerStatus represents the Mirth Connect server status response.
// GET /api/server/status
type ServerStatus struct {
	Status string `json:"status"` // e.g. "RUNNING", "STOPPED"
}

// SystemStats represents the Mirth Connect system statistics response.
// GET /api/system/stats
type SystemStats struct {
	Timestamp     string  `json:"timestamp"`
	FreeMemory    int64   `json:"freeMemoryBytes"`
	AllocMemory   int64   `json:"allocatedMemoryBytes"`
	MaxMemory     int64   `json:"maxMemoryBytes"`
	CPUUsagePct   float64 `json:"cpuUsagePct"`
	DiskFreeBytes int64   `json:"diskFreeBytes"`
}

// DashboardStatusList wraps the Mirth dashboard status list response.
// GET /api/channels/statuses
type DashboardStatusList struct {
	DashboardStatuses []DashboardStatus `json:"dashboardStatus"`
}

// DashboardStatus represents a single channel's dashboard status from Mirth.
type DashboardStatus struct {
	ChannelID     string             `json:"channelId"`
	Name          string             `json:"name"`
	State         string             `json:"state"` // STARTED, STOPPED, PAUSED, ERROR
	DeployedDate  string             `json:"deployedDate,omitempty"`
	Statistics    *ChannelStatistics `json:"statistics,omitempty"`
	ChildStatuses []ConnectorStatus  `json:"childStatuses,omitempty"`
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

// ChannelStatistics represents the message statistics for a channel.
// GET /api/channels/{id}/statistics
type ChannelStatistics struct {
	ChannelID string `json:"channelId,omitempty"`
	Received  int64  `json:"received"`
	Sent      int64  `json:"sent"`
	Error     int64  `json:"error"`
	Filtered  int64  `json:"filtered"`
	Queued    int64  `json:"queued"`
}
