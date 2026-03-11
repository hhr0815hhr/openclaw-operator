/*
Copyright 2026 OpenClaw Platform.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	openclawv1alpha1 "github.com/openclaw/operator/api/v1alpha1"
)

var _ = Describe("Health Check Controller", func() {
	Context("When checking Pod health", func() {
		ctx := context.Background()

		It("should detect running Pod and update status", func() {
			instance := &openclawv1alpha1.OpenClawInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "health-test-running",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawInstanceSpec{
					InstanceID: "health-running-001",
					UserID:     "user-health",
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			// Create mock Pod
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openclaw-health-running-001-pod",
					Namespace: "default",
					Labels: map[string]string{
						"app":        "openclaw",
						"instanceId": "health-running-001",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
					Containers: []corev1.Container{
						{Name: "openclaw", Image: "openclaw/openclaw:latest"},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "openclaw", Ready: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			By("Reconciling health check")
			reconciler := &OpenClawInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "health-test-running",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying status update")
			Eventually(func() string {
				updated := &openclawv1alpha1.OpenClawInstance{}
				k8sClient.Get(ctx, types.NamespacedName{Name: "health-test-running", Namespace: "default"}, updated)
				return updated.Status.Phase
			}, time.Second*30, time.Second).Should(Equal(openclawv1alpha1.PhaseRunning))
		})

		It("should handle pending Pod", func() {
			instance := &openclawv1alpha1.OpenClawInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "health-test-pending",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawInstanceSpec{
					InstanceID: "health-pending-001",
					UserID:     "user-health",
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openclaw-health-pending-001-pod",
					Namespace: "default",
					Labels: map[string]string{
						"app":        "openclaw",
						"instanceId": "health-pending-001",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "openclaw", Image: "openclaw/openclaw:latest"},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			reconciler := &OpenClawInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "health-test-pending",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Pending should stay in Creating phase
			Consistently(func() string {
				updated := &openclawv1alpha1.OpenClawInstance{}
				k8sClient.Get(ctx, types.NamespacedName{Name: "health-test-pending", Namespace: "default"}, updated)
				return updated.Status.Phase
			}, time.Second*5, time.Second).Should(Or(
				Equal(openclawv1alpha1.PhaseCreating),
				Equal(openclawv1alpha1.PhasePending),
			))
		})

		It("should detect failed Pod", func() {
			instance := &openclawv1alpha1.OpenClawInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "health-test-failed",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawInstanceSpec{
					InstanceID: "health-failed-001",
					UserID:     "user-health",
				},
			}
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openclaw-health-failed-001-pod",
					Namespace: "default",
					Labels: map[string]string{
						"app":        "openclaw",
						"instanceId": "health-failed-001",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "openclaw", Image: "openclaw/openclaw:latest"},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			reconciler := &OpenClawInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "health-test-failed",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				updated := &openclawv1alpha1.OpenClawInstance{}
				k8sClient.Get(ctx, types.NamespacedName{Name: "health-test-failed", Namespace: "default"}, updated)
				return updated.Status.Phase
			}, time.Second*30, time.Second).Should(Equal(openclawv1alpha1.PhaseError))
		})
	})
})
