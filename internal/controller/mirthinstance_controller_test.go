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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mirthv1alpha1 "github.com/pyle-cloud-technologies/mirth-operator/api/v1alpha1"
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
})

// fakeRecorder implements record.EventRecorder for testing.
type fakeRecorder struct{}

func (f *fakeRecorder) Event(_ runtime.Object, _, _, _ string)                    {}
func (f *fakeRecorder) Eventf(_ runtime.Object, _, _, _ string, _ ...interface{}) {}
func (f *fakeRecorder) AnnotatedEventf(_ runtime.Object, _ map[string]string, _, _, _ string, _ ...interface{}) {
}
