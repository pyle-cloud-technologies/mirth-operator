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

package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	registerOnce sync.Once
	collector    *Collector
)

// Collector holds all Prometheus metrics for Mirth instances.
type Collector struct {
	MirthUp                 *prometheus.GaugeVec
	ChannelStatus           *prometheus.GaugeVec
	ChannelMessagesReceived *prometheus.GaugeVec
	ChannelMessagesSent     *prometheus.GaugeVec
	ChannelMessagesErrored  *prometheus.GaugeVec
	ChannelMessagesQueued   *prometheus.GaugeVec
	ChannelMessagesFiltered *prometheus.GaugeVec
	ChannelsTotal           *prometheus.GaugeVec
	ChannelsHealthy         *prometheus.GaugeVec
	RemediationTotal        *prometheus.CounterVec
	JVMHeapUsedBytes        *prometheus.GaugeVec
}

// GetCollector returns the singleton metrics collector, registering it on first call.
func GetCollector() *Collector {
	registerOnce.Do(func() {
		collector = newCollector()
		metrics.Registry.MustRegister(
			collector.MirthUp,
			collector.ChannelStatus,
			collector.ChannelMessagesReceived,
			collector.ChannelMessagesSent,
			collector.ChannelMessagesErrored,
			collector.ChannelMessagesQueued,
			collector.ChannelMessagesFiltered,
			collector.ChannelsTotal,
			collector.ChannelsHealthy,
			collector.RemediationTotal,
			collector.JVMHeapUsedBytes,
		)
	})
	return collector
}

func newCollector() *Collector {
	return &Collector{
		MirthUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_up",
			Help: "Whether the Mirth Connect REST API is reachable (1=up, 0=down).",
		}, []string{"instance"}),

		ChannelStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channel_status",
			Help: "Current channel state (1 if channel is in this state, 0 otherwise).",
		}, []string{"instance", "channel", "state"}),

		ChannelMessagesReceived: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channel_messages_received_total",
			Help: "Total messages received by channel.",
		}, []string{"instance", "channel"}),

		ChannelMessagesSent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channel_messages_sent_total",
			Help: "Total messages sent by channel.",
		}, []string{"instance", "channel"}),

		ChannelMessagesErrored: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channel_messages_errored_total",
			Help: "Total errored messages by channel.",
		}, []string{"instance", "channel"}),

		ChannelMessagesQueued: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channel_messages_queued",
			Help: "Current queue depth by channel.",
		}, []string{"instance", "channel"}),

		ChannelMessagesFiltered: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channel_messages_filtered_total",
			Help: "Total filtered messages by channel.",
		}, []string{"instance", "channel"}),

		ChannelsTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channels_total",
			Help: "Total number of channels.",
		}, []string{"instance"}),

		ChannelsHealthy: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_channels_healthy",
			Help: "Number of channels in STARTED state.",
		}, []string{"instance"}),

		RemediationTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mirth_remediation_total",
			Help: "Total remediation attempts.",
		}, []string{"instance", "channel", "result"}),

		JVMHeapUsedBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mirth_jvm_heap_used_bytes",
			Help: "JVM heap memory used in bytes.",
		}, []string{"instance"}),
	}
}

// ResetInstance removes all metrics for a given instance name.
func (c *Collector) ResetInstance(instance string) {
	c.MirthUp.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelStatus.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelMessagesReceived.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelMessagesSent.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelMessagesErrored.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelMessagesQueued.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelMessagesFiltered.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelsTotal.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.ChannelsHealthy.DeletePartialMatch(prometheus.Labels{"instance": instance})
	c.JVMHeapUsedBytes.DeletePartialMatch(prometheus.Labels{"instance": instance})
}
