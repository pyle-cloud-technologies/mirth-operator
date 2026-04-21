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

//nolint:goconst // test values are more readable as literals
package remediation

import (
	"context"
	"fmt"
	"testing"
	"time"

	mirthv1alpha1 "github.com/pyle-cloud-technologies/mirth-operator/api/v1alpha1"
	"github.com/pyle-cloud-technologies/mirth-operator/internal/mirth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockClient implements mirth.Client for testing.
type mockClient struct {
	startCalled   map[string]int
	restartCalled map[string]int
	failChannels  map[string]bool
}

func newMockClient() *mockClient {
	return &mockClient{
		startCalled:   make(map[string]int),
		restartCalled: make(map[string]int),
		failChannels:  make(map[string]bool),
	}
}

func (m *mockClient) GetServerStatus(_ context.Context) (*mirth.ServerStatusResponse, error) {
	return &mirth.ServerStatusResponse{Int: 0}, nil
}

func (m *mockClient) GetSystemStats(_ context.Context) (*mirth.SystemStats, error) {
	return &mirth.SystemStats{}, nil
}

func (m *mockClient) GetChannelStatuses(_ context.Context) ([]mirth.DashboardStatus, error) {
	return nil, nil
}

func (m *mockClient) GetChannelStatistics(_ context.Context, _ string) (*mirth.ChannelStatistics, error) {
	return &mirth.ChannelStatistics{}, nil
}

func (m *mockClient) GetEvents(_ context.Context, _ int64, _ int) ([]mirth.ServerEvent, error) {
	return nil, nil
}

func (m *mockClient) StartChannel(_ context.Context, id string) error {
	m.startCalled[id]++
	if m.failChannels[id] {
		return fmt.Errorf("failed to start channel %s", id)
	}
	return nil
}

func (m *mockClient) RestartChannel(_ context.Context, id string) error {
	m.restartCalled[id]++
	if m.failChannels[id] {
		return fmt.Errorf("failed to restart channel %s", id)
	}
	return nil
}

func TestEvaluate_DisabledRemediation(t *testing.T) {
	handler := NewHandler(newMockClient())
	spec := mirthv1alpha1.RemediationSpec{Enabled: false}

	actions := handler.Evaluate(spec, []mirth.DashboardStatus{
		{ChannelID: "ch-1", Name: "Test", State: "STOPPED"},
	}, nil)

	if len(actions) != 0 {
		t.Errorf("expected no actions when disabled, got %d", len(actions))
	}
}

func TestEvaluate_RestartStoppedChannels(t *testing.T) {
	handler := NewHandler(newMockClient())
	spec := mirthv1alpha1.RemediationSpec{
		Enabled:                true,
		RestartStoppedChannels: true,
		MaxRestartAttempts:     3,
		CooldownSeconds:        60,
	}

	channels := []mirth.DashboardStatus{
		{ChannelID: "ch-1", Name: "Started Channel", State: "STARTED"},
		{ChannelID: "ch-2", Name: "Stopped Channel", State: "STOPPED"},
		{ChannelID: "ch-3", Name: "Paused Channel", State: "PAUSED"},
	}

	actions := handler.Evaluate(spec, channels, nil)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ChannelID != "ch-2" {
		t.Errorf("expected ch-2, got %s", actions[0].ChannelID)
	}
	if actions[0].ActionType != "start" {
		t.Errorf("expected start action, got %s", actions[0].ActionType)
	}
}

func TestEvaluate_RestartErroredChannels(t *testing.T) {
	handler := NewHandler(newMockClient())
	spec := mirthv1alpha1.RemediationSpec{
		Enabled:                true,
		RestartErroredChannels: true,
		MaxRestartAttempts:     3,
		CooldownSeconds:        60,
	}

	channels := []mirth.DashboardStatus{
		{ChannelID: "ch-1", Name: "OK Channel", State: "STARTED"},
		{ChannelID: "ch-2", Name: "Error Channel", State: "ERROR"},
	}

	actions := handler.Evaluate(spec, channels, nil)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ActionType != "restart" {
		t.Errorf("expected restart action, got %s", actions[0].ActionType)
	}
}

func TestEvaluate_ExcludeChannels(t *testing.T) {
	handler := NewHandler(newMockClient())
	spec := mirthv1alpha1.RemediationSpec{
		Enabled:                true,
		RestartStoppedChannels: true,
		ExcludeChannels:        []string{"ch-2", "Excluded By Name"},
	}

	channels := []mirth.DashboardStatus{
		{ChannelID: "ch-1", Name: "Should Restart", State: "STOPPED"},
		{ChannelID: "ch-2", Name: "Excluded By ID", State: "STOPPED"},
		{ChannelID: "ch-3", Name: "Excluded By Name", State: "STOPPED"},
	}

	actions := handler.Evaluate(spec, channels, nil)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ChannelID != "ch-1" {
		t.Errorf("expected ch-1, got %s", actions[0].ChannelID)
	}
}

func TestEvaluate_Cooldown(t *testing.T) {
	handler := NewHandler(newMockClient())
	spec := mirthv1alpha1.RemediationSpec{
		Enabled:                true,
		RestartStoppedChannels: true,
		CooldownSeconds:        300,
	}

	channels := []mirth.DashboardStatus{
		{ChannelID: "ch-1", Name: "Recently Restarted", State: "STOPPED"},
		{ChannelID: "ch-2", Name: "Long Ago Restarted", State: "STOPPED"},
	}

	history := []mirthv1alpha1.RemediationEvent{
		{
			ChannelID: "ch-1",
			Action:    "start",
			Result:    "success",
			Timestamp: metav1.NewTime(time.Now().Add(-1 * time.Minute)), // 1 min ago (within cooldown)
		},
		{
			ChannelID: "ch-2",
			Action:    "start",
			Result:    "success",
			Timestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)), // 10 min ago (past cooldown)
		},
	}

	actions := handler.Evaluate(spec, channels, history)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ChannelID != "ch-2" {
		t.Errorf("expected ch-2 (past cooldown), got %s", actions[0].ChannelID)
	}
}

func TestEvaluate_MaxRestartAttempts(t *testing.T) {
	handler := NewHandler(newMockClient())
	spec := mirthv1alpha1.RemediationSpec{
		Enabled:                true,
		RestartStoppedChannels: true,
		MaxRestartAttempts:     2,
		CooldownSeconds:        0,
	}

	channels := []mirth.DashboardStatus{
		{ChannelID: "ch-1", Name: "Exhausted", State: "STOPPED"},
		{ChannelID: "ch-2", Name: "Has Room", State: "STOPPED"},
	}

	history := []mirthv1alpha1.RemediationEvent{
		{ChannelID: "ch-1", Action: "start", Result: "failure", Timestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute))},
		{ChannelID: "ch-1", Action: "start", Result: "failure", Timestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute))},
		{ChannelID: "ch-2", Action: "start", Result: "failure", Timestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute))},
	}

	actions := handler.Evaluate(spec, channels, history)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ChannelID != "ch-2" {
		t.Errorf("expected ch-2 (still has attempts), got %s", actions[0].ChannelID)
	}
}

func TestExecute_StartSuccess(t *testing.T) {
	mc := newMockClient()
	handler := NewHandler(mc)

	event := handler.Execute(context.Background(), Action{
		ChannelID:   "ch-1",
		ChannelName: "Test Channel",
		ActionType:  "start",
	})

	if event.Result != "success" {
		t.Errorf("expected success, got %s", event.Result)
	}
	if mc.startCalled["ch-1"] != 1 {
		t.Errorf("expected StartChannel called once, got %d", mc.startCalled["ch-1"])
	}
}

func TestExecute_RestartSuccess(t *testing.T) {
	mc := newMockClient()
	handler := NewHandler(mc)

	event := handler.Execute(context.Background(), Action{
		ChannelID:   "ch-1",
		ChannelName: "Test Channel",
		ActionType:  "restart",
	})

	if event.Result != "success" {
		t.Errorf("expected success, got %s", event.Result)
	}
	if mc.restartCalled["ch-1"] != 1 {
		t.Errorf("expected RestartChannel called once, got %d", mc.restartCalled["ch-1"])
	}
}

func TestExecute_Failure(t *testing.T) {
	mc := newMockClient()
	mc.failChannels["ch-1"] = true
	handler := NewHandler(mc)

	event := handler.Execute(context.Background(), Action{
		ChannelID:   "ch-1",
		ChannelName: "Broken Channel",
		ActionType:  "start",
	})

	if event.Result != "failure" {
		t.Errorf("expected failure, got %s", event.Result)
	}
	if event.Message == "" {
		t.Error("expected error message")
	}
}
