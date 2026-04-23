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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mirthv1alpha1 "github.com/pyle-cloud-technologies/mirth-operator/api/v1alpha1"
	"github.com/pyle-cloud-technologies/mirth-operator/internal/metrics"
	mirthclient "github.com/pyle-cloud-technologies/mirth-operator/internal/mirth"
	"github.com/pyle-cloud-technologies/mirth-operator/internal/remediation"
)

const maxRemediationHistory = 50

// MirthInstanceReconciler reconciles a MirthInstance object.
type MirthInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// lastEventID tracks the highest OIE event id already observed for each
	// instance, so /api/events polling only returns new events per reconcile.
	// State lives only for the operator process lifetime; on restart a
	// bounded replay is fine because DeployErrorsTotal is a counter.
	lastEventIDMu sync.Mutex
	lastEventID   map[types.NamespacedName]int64
}

// +kubebuilder:rbac:groups=mirth.pyle.io,resources=mirthinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mirth.pyle.io,resources=mirthinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mirth.pyle.io,resources=mirthinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *MirthInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the MirthInstance CR
	var instance mirthv1alpha1.MirthInstance
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	instanceName := fmt.Sprintf("%s/%s", instance.Namespace, instance.Name)
	collector := metrics.GetCollector()

	// 2. Read the auth secret
	var secret corev1.Secret
	secretRef := types.NamespacedName{
		Name:      instance.Spec.Connection.AuthSecretRef.Name,
		Namespace: instance.Namespace,
	}
	if err := r.Get(ctx, secretRef, &secret); err != nil {
		log.Error(err, "Failed to get auth secret", "secret", secretRef)
		r.setCondition(&instance, "Connected", metav1.ConditionFalse, "SecretNotFound", "Auth secret not found: "+err.Error())
		collector.MirthUp.WithLabelValues(instanceName).Set(0)
		return r.updateStatusAndRequeue(ctx, &instance)
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	if username == "" || password == "" {
		log.Error(nil, "Auth secret missing username or password keys", "secret", secretRef)
		r.setCondition(&instance, "Connected", metav1.ConditionFalse, "InvalidSecret", "Secret must contain 'username' and 'password' keys")
		collector.MirthUp.WithLabelValues(instanceName).Set(0)
		return r.updateStatusAndRequeue(ctx, &instance)
	}

	// 3. Build Mirth client
	port := instance.Spec.Connection.Port
	if port == 0 {
		port = 8443
	}
	mirthCli := mirthclient.NewClient(mirthclient.ClientConfig{
		BaseURL:            fmt.Sprintf("https://%s:%d", instance.Spec.Connection.Host, port),
		Username:           username,
		Password:           password,
		InsecureSkipVerify: instance.Spec.Connection.TLS.InsecureSkipVerify,
	})

	// 4. Poll server status
	serverStatus, err := mirthCli.GetServerStatus(ctx)
	if err != nil {
		log.Error(err, "Failed to connect to Mirth")
		r.setCondition(&instance, "Connected", metav1.ConditionFalse, "ConnectionFailed", err.Error())
		collector.MirthUp.WithLabelValues(instanceName).Set(0)
		return r.updateStatusAndRequeue(ctx, &instance)
	}

	collector.MirthUp.WithLabelValues(instanceName).Set(1)
	r.setCondition(&instance, "Connected", metav1.ConditionTrue, "Connected", "Successfully connected to Mirth")
	instance.Status.Server.Status = serverStatus.ServerStatusString()

	// 5. Poll system stats
	sysStats, err := mirthCli.GetSystemStats(ctx)
	if err != nil {
		log.Error(err, "Failed to get system stats")
	} else {
		heapUsed := sysStats.AllocMemory - sysStats.FreeMemory
		instance.Status.Server.JVMHeapUsedBytes = heapUsed
		collector.JVMHeapUsedBytes.WithLabelValues(instanceName).Set(float64(heapUsed))
	}

	// 6. Poll channel statuses
	channelStatuses, err := mirthCli.GetChannelStatuses(ctx)
	if err != nil {
		log.Error(err, "Failed to get channel statuses")
		r.setCondition(&instance, "AllChannelsHealthy", metav1.ConditionUnknown, "PollFailed", err.Error())
		return r.updateStatusAndRequeue(ctx, &instance)
	}

	// 7. Update channel summary and details
	summary := mirthv1alpha1.ChannelSummary{Total: len(channelStatuses)}
	details := make([]mirthv1alpha1.ChannelStatus, 0, len(channelStatuses))
	states := []string{"STARTED", "STOPPED", "PAUSED", "ERROR"}

	for _, ch := range channelStatuses {
		switch ch.State {
		case "STARTED":
			summary.Started++
		case "STOPPED":
			summary.Stopped++
		case "ERROR":
			summary.Errored++
		case "PAUSED":
			summary.Paused++
		}

		stats := ch.ParseStatistics()

		detail := mirthv1alpha1.ChannelStatus{
			ID:       ch.ChannelID,
			Name:     ch.Name,
			State:    ch.State,
			Received: stats.Received,
			Sent:     stats.Sent,
			Errored:  stats.Error,
			Queued:   stats.Queued,
			Filtered: stats.Filtered,
		}

		details = append(details, detail)

		// 8. Update per-channel metrics
		if instance.Spec.Monitoring.Metrics.Enabled {
			for _, s := range states {
				val := float64(0)
				if ch.State == s {
					val = 1
				}
				collector.ChannelStatus.WithLabelValues(instanceName, ch.Name, s).Set(val)
			}

			collector.ChannelMessagesReceived.WithLabelValues(instanceName, ch.Name).Set(float64(stats.Received))
			collector.ChannelMessagesSent.WithLabelValues(instanceName, ch.Name).Set(float64(stats.Sent))
			collector.ChannelMessagesErrored.WithLabelValues(instanceName, ch.Name).Set(float64(stats.Error))
			collector.ChannelMessagesQueued.WithLabelValues(instanceName, ch.Name).Set(float64(stats.Queued))
			collector.ChannelMessagesFiltered.WithLabelValues(instanceName, ch.Name).Set(float64(stats.Filtered))

			// 8b. Update per-destination metrics
			children := ch.ParseChildStatuses()
			for _, child := range children {
				if child.MetaDataID == 0 {
					continue // skip source connector, only expose destinations
				}
				destStats := child.ParseStatistics()
				collector.DestinationMessagesReceived.WithLabelValues(instanceName, ch.Name, child.Name).Set(float64(destStats.Received))
				collector.DestinationMessagesSent.WithLabelValues(instanceName, ch.Name, child.Name).Set(float64(destStats.Sent))
				collector.DestinationMessagesErrored.WithLabelValues(instanceName, ch.Name, child.Name).Set(float64(destStats.Error))
				collector.DestinationMessagesQueued.WithLabelValues(instanceName, ch.Name, child.Name).Set(float64(destStats.Queued))
				collector.DestinationMessagesFiltered.WithLabelValues(instanceName, ch.Name, child.Name).Set(float64(destStats.Filtered))
			}
		}
	}

	instance.Status.Channels = summary
	instance.Status.ChannelDetails = details

	if instance.Spec.Monitoring.Metrics.Enabled {
		collector.ChannelsTotal.WithLabelValues(instanceName).Set(float64(summary.Total))
		collector.ChannelsHealthy.WithLabelValues(instanceName).Set(float64(summary.Started))
	}

	// 9. Poll /api/events for deploy/script/compile errors. A channel can
	// report STARTED while holding a broken script; the events endpoint is
	// the only signal that surfaces those failures.
	if instance.Spec.Monitoring.Events.Enabled {
		r.pollEvents(ctx, req.NamespacedName, mirthCli, &instance, instanceName, collector)
	}

	// 10. Evaluate health
	allHealthy := summary.Total > 0 && summary.Started == summary.Total
	if allHealthy {
		r.setCondition(&instance, "AllChannelsHealthy", metav1.ConditionTrue, "AllStarted", "All channels are in STARTED state")
	} else {
		r.setCondition(&instance, "AllChannelsHealthy", metav1.ConditionFalse, "UnhealthyChannels",
			fmt.Sprintf("%d/%d channels not started (stopped=%d, errored=%d, paused=%d)",
				summary.Total-summary.Started, summary.Total, summary.Stopped, summary.Errored, summary.Paused))
	}

	// 11. Remediation
	if instance.Spec.Remediation.Enabled {
		r.setCondition(&instance, "RemediationActive", metav1.ConditionTrue, "Enabled", "Automatic remediation is active")

		handler := remediation.NewHandler(mirthCli)
		actions := handler.Evaluate(instance.Spec.Remediation, channelStatuses, instance.Status.RemediationHistory)

		for _, action := range actions {
			event := handler.Execute(ctx, action)
			instance.Status.RemediationHistory = append(instance.Status.RemediationHistory, event)

			// Emit K8s event
			eventType := corev1.EventTypeNormal
			if event.Result == "failure" {
				eventType = corev1.EventTypeWarning
			}
			r.Recorder.Event(&instance, eventType, "Remediation",
				fmt.Sprintf("%s channel %s (%s): %s", event.Action, event.ChannelName, event.ChannelID, event.Result))

			// Update metrics
			if instance.Spec.Monitoring.Metrics.Enabled {
				collector.RemediationTotal.WithLabelValues(instanceName, event.ChannelName, event.Result).Inc()
			}

			log.Info("Remediation executed",
				"channel", event.ChannelName,
				"action", event.Action,
				"result", event.Result)
		}

		// Trim history
		if len(instance.Status.RemediationHistory) > maxRemediationHistory {
			instance.Status.RemediationHistory = instance.Status.RemediationHistory[len(instance.Status.RemediationHistory)-maxRemediationHistory:]
		}
	} else {
		r.setCondition(&instance, "RemediationActive", metav1.ConditionFalse, "Disabled", "Automatic remediation is disabled")
	}

	// 12. Update status
	now := metav1.Now()
	instance.Status.LastChecked = &now

	return r.updateStatusAndRequeue(ctx, &instance)
}

// pollEvents fetches new OIE server events since the last observed ID,
// increments DeployErrorsTotal for each event classified as a deploy or
// script error, and updates the DeployErrorsDetected condition. Failures
// are logged but non-fatal — the rest of the reconcile is still useful.
func (r *MirthInstanceReconciler) pollEvents(
	ctx context.Context,
	key types.NamespacedName,
	cli mirthclient.Client,
	instance *mirthv1alpha1.MirthInstance,
	instanceName string,
	collector *metrics.Collector,
) {
	log := logf.FromContext(ctx)

	limit := instance.Spec.Monitoring.Events.LookbackLimit
	if limit <= 0 {
		limit = 100
	}

	r.lastEventIDMu.Lock()
	if r.lastEventID == nil {
		r.lastEventID = make(map[types.NamespacedName]int64)
	}
	cursor := r.lastEventID[key]
	r.lastEventIDMu.Unlock()

	events, err := cli.GetEvents(ctx, cursor, limit)
	if err != nil {
		log.Error(err, "Failed to poll Mirth /api/events; deploy-error detection disabled for this reconcile")
		return
	}

	var (
		errorCount int
		maxID      = cursor
	)
	for _, e := range events {
		if e.ID > maxID {
			maxID = e.ID
		}
		if !e.IsDeployError() {
			continue
		}
		errorCount++
		if instance.Spec.Monitoring.Metrics.Enabled {
			_, channelName := e.ChannelRef()
			if channelName == "" {
				channelName = "<unknown>"
			}
			collector.DeployErrorsTotal.WithLabelValues(instanceName, channelName, e.Name).Inc()
		}
	}

	if maxID > cursor {
		r.lastEventIDMu.Lock()
		r.lastEventID[key] = maxID
		r.lastEventIDMu.Unlock()
	}

	if errorCount > 0 {
		r.setCondition(instance, "DeployErrorsDetected", metav1.ConditionTrue, "ErrorEventsObserved",
			fmt.Sprintf("%d deploy/script error event(s) observed via /api/events", errorCount))
	} else {
		r.setCondition(instance, "DeployErrorsDetected", metav1.ConditionFalse, "NoErrorEvents",
			"No deploy/script error events observed in the latest /api/events window")
	}
}

func (r *MirthInstanceReconciler) updateStatusAndRequeue(ctx context.Context, instance *mirthv1alpha1.MirthInstance) (ctrl.Result, error) {
	if err := r.Status().Update(ctx, instance); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update status")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	interval := instance.Spec.Monitoring.IntervalSeconds
	if interval <= 0 {
		interval = 30
	}
	return ctrl.Result{RequeueAfter: time.Duration(interval) * time.Second}, nil
}

func (r *MirthInstanceReconciler) setCondition(instance *mirthv1alpha1.MirthInstance, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: instance.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *MirthInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.lastEventID == nil {
		r.lastEventID = make(map[types.NamespacedName]int64)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&mirthv1alpha1.MirthInstance{}).
		Named("mirthinstance").
		Complete(r)
}
