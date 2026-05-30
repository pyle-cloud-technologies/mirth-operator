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
	"testing"
)

func TestChannelType(t *testing.T) {
	tests := []struct {
		transport string
		want      string
	}{
		{"HTTP Listener", "http_inbound"},
		{"Web Service Listener", "http_inbound"},
		{"HTTP Sender", "http_outbound"},
		{"TCP Listener", "tcp_inbound"},
		{"LLP Listener", "tcp_inbound"},
		{"MLLP Listener", "tcp_inbound"},
		{"HL7 v2.x Listener", "tcp_inbound"},
		{"TCP Sender", "tcp_outbound"},
		{"Channel Reader", "internal"},
		{"Database Reader", "db_inbound"},
		{"Database Writer", "db_outbound"},
		{"File Reader", "file_inbound"},
		{"JavaScript Reader", "polling"},
		{"JMS Listener", "jms_inbound"},
		{"SMTP Sender", "smtp_outbound"},
		{"", "unknown"},
		{"Some Future Connector", "other"},
	}
	for _, tt := range tests {
		t.Run(tt.transport, func(t *testing.T) {
			ch := Channel{SourceConnector: Connector{TransportName: tt.transport}}
			if got := ch.ChannelType(); got != tt.want {
				t.Errorf("ChannelType(%q) = %q; want %q", tt.transport, got, tt.want)
			}
		})
	}
}

func TestDirection(t *testing.T) {
	tests := []struct {
		transport string
		want      string
	}{
		{"HTTP Listener", "inbound"},
		{"Channel Reader", "inbound"},
		{"Database Reader", "inbound"},
		{"JavaScript Reader", "inbound"},
		{"HTTP Sender", "outbound"},
		{"TCP Sender", "outbound"},
		{"SMTP Sender", "outbound"},
		{"", "unknown"},
		{"Future Thing", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.transport, func(t *testing.T) {
			ch := Channel{SourceConnector: Connector{TransportName: tt.transport}}
			if got := ch.Direction(); got != tt.want {
				t.Errorf("Direction(%q) = %q; want %q", tt.transport, got, tt.want)
			}
		})
	}
}

func TestStorageMode(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{"DEVELOPMENT", "development"},
		{"PRODUCTION", "production"},
		{"RAW", "raw"},
		{"METADATA", "metadata"},
		{"DISABLED", "disabled"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			ch := Channel{Properties: ChannelProperties{MessageStorageMode: tt.mode}}
			if got := ch.StorageMode(); got != tt.want {
				t.Errorf("StorageMode(%q) = %q; want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestPartner(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		want      string
	}{
		// Name-based matches across known partners (case-insensitive, substring).
		{"BetterRX", "HTTP Sender", "betterrx"},
		{"better_rx production", "HTTP Sender", "betterrx"},
		{"OnePoint", "HTTP Sender", "onepoint"},
		{"OPPC", "HTTP Sender", "onepoint"},
		{"dragonfly", "HTTP Sender", "dragonfly"},
		{"Enclara Pharmacia", "HTTP Sender", "enclara"},
		{"ScriptSure Mailbox", "HTTP Sender", "scriptsure"},
		{"Waystar SFTP", "HTTP Sender", "waystar"},
		{"Ability Network", "HTTP Sender", "ability"},
		{"Qualis Census", "HTTP Sender", "qualis"},
		{"Echo Dump", "Channel Writer", "echo"},
		{"PDC Rx", "HTTP Sender", "procare-or-pdc"},
		{"ProCare Rx", "HTTP Sender", "procare-or-pdc"},
		// Transport-based fallback.
		{"Generic Internal", "Channel Writer", "internal"},
		{"Unknown Vendor", "HTTP Sender", "other"},
		{"Anonymous", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Connector{Name: tt.name, TransportName: tt.transport}
			if got := Partner(d); got != tt.want {
				t.Errorf("Partner(name=%q, transport=%q) = %q; want %q", tt.name, tt.transport, got, tt.want)
			}
		})
	}
}

func TestDestinationWrapper_BothShapes(t *testing.T) {
	// Mirth serializes a single destination as an object, multiple as an array.
	// Destinations() must handle both.
	t.Run("array shape", func(t *testing.T) {
		raw := json.RawMessage(`[
			{"name": "Dest A", "transportName": "HTTP Sender", "metaDataId": 1},
			{"name": "Dest B", "transportName": "Channel Writer", "metaDataId": 2}
		]`)
		w := DestinationWrapper{Raw: raw}
		dests := w.Destinations()
		if len(dests) != 2 {
			t.Fatalf("expected 2 destinations, got %d", len(dests))
		}
		if dests[0].Name != "Dest A" || dests[1].Name != "Dest B" {
			t.Errorf("destination names wrong: %+v", dests)
		}
	})

	t.Run("single-object shape", func(t *testing.T) {
		raw := json.RawMessage(`{"name": "Only Dest", "transportName": "JavaScript Writer", "metaDataId": 1}`)
		w := DestinationWrapper{Raw: raw}
		dests := w.Destinations()
		if len(dests) != 1 {
			t.Fatalf("expected 1 destination, got %d", len(dests))
		}
		if dests[0].Name != "Only Dest" {
			t.Errorf("destination name wrong: %+v", dests)
		}
	})

	t.Run("empty", func(t *testing.T) {
		w := DestinationWrapper{}
		if got := w.Destinations(); got != nil {
			t.Errorf("expected nil for empty raw, got %+v", got)
		}
	})
}
