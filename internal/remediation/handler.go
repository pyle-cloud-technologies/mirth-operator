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

package remediation

import (
	"context"
	"fmt"
	"time"

	mirthv1alpha1 "github.com/pyle-cloud-technologies/mirth-operator/api/v1alpha1"
	"github.com/pyle-cloud-technologies/mirth-operator/internal/mirth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Handler evaluates and executes channel remediation actions.
type Handler struct {
	mirthClient mirth.Client
}

// NewHandler creates a new remediation handler.
func NewHandler(mirthClient mirth.Client) *Handler {
	return &Handler{
		mirthClient: mirthClient,
	}
}

// Action represents a planned remediation action.
type Action struct {
	ChannelID   string
	ChannelName string
	ActionType  string // "start" or "restart"
}

// Evaluate determines which channels need remediation based on the spec and current state.
func (h *Handler) Evaluate(
	spec mirthv1alpha1.RemediationSpec,
	channels []mirth.DashboardStatus,
	history []mirthv1alpha1.RemediationEvent,
) []Action {
	if !spec.Enabled {
		return nil
	}

	excludeSet := make(map[string]bool, len(spec.ExcludeChannels))
	for _, ch := range spec.ExcludeChannels {
		excludeSet[ch] = true
	}

	now := time.Now()
	var actions []Action

	for _, ch := range channels {
		// Skip excluded channels (by ID or name)
		if excludeSet[ch.ChannelID] || excludeSet[ch.Name] {
			continue
		}

		var actionType string
		switch {
		case ch.State == "STOPPED" && spec.RestartStoppedChannels:
			actionType = "start"
		case ch.State == "ERROR" && spec.RestartErroredChannels:
			actionType = "restart"
		default:
			continue
		}

		// Check cooldown
		if h.isInCooldown(ch.ChannelID, history, spec.CooldownSeconds, now) {
			continue
		}

		// Check max attempts
		if spec.MaxRestartAttempts > 0 && h.attemptCount(ch.ChannelID, history) >= spec.MaxRestartAttempts {
			continue
		}

		actions = append(actions, Action{
			ChannelID:   ch.ChannelID,
			ChannelName: ch.Name,
			ActionType:  actionType,
		})
	}

	return actions
}

// Execute performs a single remediation action and returns the result event.
func (h *Handler) Execute(ctx context.Context, action Action) mirthv1alpha1.RemediationEvent {
	now := metav1.Now()
	event := mirthv1alpha1.RemediationEvent{
		ChannelID:   action.ChannelID,
		ChannelName: action.ChannelName,
		Action:      action.ActionType,
		Timestamp:   now,
	}

	var err error
	switch action.ActionType {
	case "start":
		err = h.mirthClient.StartChannel(ctx, action.ChannelID)
	case "restart":
		err = h.mirthClient.RestartChannel(ctx, action.ChannelID)
	default:
		err = fmt.Errorf("unknown action type: %s", action.ActionType)
	}

	if err != nil {
		event.Result = "failure"
		event.Message = err.Error()
	} else {
		event.Result = "success"
		event.Message = fmt.Sprintf("Successfully %sed channel %s (%s)", action.ActionType, action.ChannelName, action.ChannelID)
	}

	return event
}

func (h *Handler) isInCooldown(channelID string, history []mirthv1alpha1.RemediationEvent, cooldownSeconds int, now time.Time) bool {
	if cooldownSeconds <= 0 {
		return false
	}

	cooldown := time.Duration(cooldownSeconds) * time.Second
	for i := len(history) - 1; i >= 0; i-- {
		e := history[i]
		if e.ChannelID == channelID {
			if now.Sub(e.Timestamp.Time) < cooldown {
				return true
			}
			return false // only check the most recent event for this channel
		}
	}
	return false
}

func (h *Handler) attemptCount(channelID string, history []mirthv1alpha1.RemediationEvent) int {
	count := 0
	for _, e := range history {
		if e.ChannelID == channelID {
			count++
		}
	}
	return count
}
