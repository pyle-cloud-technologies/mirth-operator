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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mirthv1alpha1 "github.com/pyle-cloud-technologies/mirth-operator/api/v1alpha1"
	"github.com/pyle-cloud-technologies/mirth-operator/internal/metrics"
)

var _ = Describe("MirthInstance Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-mirth"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		mirthinstance := &mirthv1alpha1.MirthInstance{}

		BeforeEach(func() {
			// Create the auth secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mirth-test-credentials",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("admin"),
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}

			// Create the MirthInstance CR
			err = k8sClient.Get(ctx, typeNamespacedName, mirthinstance)
			if err != nil && errors.IsNotFound(err) {
				resource := &mirthv1alpha1.MirthInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: mirthv1alpha1.MirthInstanceSpec{
						Connection: mirthv1alpha1.ConnectionSpec{
							Host: "localhost",
							Port: 8443,
							TLS: mirthv1alpha1.TLSSpec{
								InsecureSkipVerify: true,
							},
							AuthSecretRef: mirthv1alpha1.SecretReference{
								Name: "mirth-test-credentials",
							},
						},
						Monitoring: mirthv1alpha1.MonitoringSpec{
							IntervalSeconds: 30,
							Metrics: mirthv1alpha1.MetricsSpec{
								Enabled: true,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &mirthv1alpha1.MirthInstance{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should set Connected=False when Mirth is unreachable", func() {
			controllerReconciler := &MirthInstanceReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: &fakeRecorder{},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// The reconciler should not return an error -- it handles connection failures gracefully
			Expect(err).NotTo(HaveOccurred())

			// Verify status was updated
			updated := &mirthv1alpha1.MirthInstance{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			// Should have a Connected=False condition since there's no real Mirth server
			var connCondition *metav1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == "Connected" {
					connCondition = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(connCondition).NotTo(BeNil())
			Expect(connCondition.Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Context("When events polling is enabled and Mirth reports a deploy error", func() {
		const resourceName = "test-mirth-events"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		var server *httptest.Server

		BeforeEach(func() {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/server/status", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"int":0}`))
			})
			mux.HandleFunc("GET /api/system/stats", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"com.mirth.connect.model.SystemStats":{"freeMemoryBytes":100,"allocatedMemoryBytes":500,"maxMemoryBytes":1000,"cpuUsagePct":0.1,"diskFreeBytes":0,"diskTotalBytes":0}}`))
			})
			mux.HandleFunc("GET /api/channels/statuses", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"list":{"dashboardStatus":[]}}`))
			})
			mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"list":{"event":[{"id":101,"level":"ERROR","name":"Channel Deployed","outcome":"FAILURE","userId":1,"dateTime":"2026-04-21T10:00:00Z","attributes":{"channelId":"ch-99","channelName":"Broken Transformer"}}]}}`))
			})

			server = httptest.NewTLSServer(mux)

			u, err := url.Parse(server.URL)
			Expect(err).NotTo(HaveOccurred())
			host, portStr, err := net.SplitHostPort(u.Host)
			Expect(err).NotTo(HaveOccurred())
			port, err := strconv.Atoi(portStr)
			Expect(err).NotTo(HaveOccurred())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mirth-events-credentials",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("admin"),
				},
			}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}

			resource := &mirthv1alpha1.MirthInstance{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &mirthv1alpha1.MirthInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: mirthv1alpha1.MirthInstanceSpec{
						Connection: mirthv1alpha1.ConnectionSpec{
							Host: host,
							Port: port,
							TLS: mirthv1alpha1.TLSSpec{
								InsecureSkipVerify: true,
							},
							AuthSecretRef: mirthv1alpha1.SecretReference{
								Name: "mirth-events-credentials",
							},
						},
						Monitoring: mirthv1alpha1.MonitoringSpec{
							IntervalSeconds: 30,
							Metrics: mirthv1alpha1.MetricsSpec{
								Enabled: true,
							},
							Events: mirthv1alpha1.EventsSpec{
								Enabled:       true,
								LookbackLimit: 100,
							},
						},
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
			resource := &mirthv1alpha1.MirthInstance{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
			metrics.GetCollector().ResetInstance("default/" + resourceName)
			metrics.GetCollector().DeployErrorsTotal.Reset()
		})

		It("increments DeployErrorsTotal and sets DeployErrorsDetected=True", func() {
			reconciler := &MirthInstanceReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: &fakeRecorder{},
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			counter := metrics.GetCollector().DeployErrorsTotal.WithLabelValues(
				"default/"+resourceName, "Broken Transformer", "Channel Deployed")
			Expect(testutil.ToFloat64(counter)).To(Equal(1.0))

			updated := &mirthv1alpha1.MirthInstance{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			var cond *metav1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == "DeployErrorsDetected" {
					cond = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		})
	})
})

// fakeRecorder implements record.EventRecorder for testing.
type fakeRecorder struct{}

func (f *fakeRecorder) Event(_ runtime.Object, _, _, _ string)            {}
func (f *fakeRecorder) Eventf(_ runtime.Object, _, _, _ string, _ ...any) {}
func (f *fakeRecorder) AnnotatedEventf(_ runtime.Object, _ map[string]string, _, _, _ string, _ ...any) {
}
