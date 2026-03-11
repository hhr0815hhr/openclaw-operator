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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openclawv1alpha1 "github.com/openclaw/operator/api/v1alpha1"
)

var _ = Describe("OpenClawInstance Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		openclawinstance := &openclawv1alpha1.OpenClawInstance{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind OpenClawInstance")
			err := k8sClient.Get(ctx, typeNamespacedName, openclawinstance)
			if err != nil && errors.IsNotFound(err) {
				resource := &openclawv1alpha1.OpenClawInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: openclawv1alpha1.OpenClawInstanceSpec{
						InstanceID: "test-instance-001",
						UserID:     "user-123456",
						Image:      "openclaw/openclaw:latest",
						Plan:       "free",
						Resources: openclawv1alpha1.ResourceRequirements{
							CPU:         "250m",
							Memory:      "512Mi",
							CPULimit:    "1000m",
							MemoryLimit: "2Gi",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("removing the custom resource")
			resource := &openclawv1alpha1.OpenClawInstance{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &OpenClawInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if Deployment was created")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "openclaw-test-instance-001",
					Namespace: "default",
				}, deployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Verifying Deployment configuration")
			Expect(deployment.Spec.Replicas).To(Equal(int32Ptr(1)))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("openclaw/openclaw:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(ContainElement(corev1.ContainerPort{
				Name:          "gateway",
				ContainerPort: 18789,
				HostPort:      18789,
				Protocol:      corev1.ProtocolTCP,
			}))

			By("Checking resource requirements")
			resources := deployment.Spec.Template.Spec.Containers[0].Resources
			Expect(resources.Requests.Cpu().String()).To(Equal("250m"))
			Expect(resources.Requests.Memory().String()).To(Equal("512Mi"))
			Expect(resources.Limits.Cpu().String()).To(Equal("1000m"))
			Expect(resources.Limits.Memory().String()).To(Equal("2Gi"))
		})

		It("should update status when Pod is running", func() {
			By("Creating a mock running Pod")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openclaw-test-pod",
					Namespace: "default",
					Labels: map[string]string{
						"app":        "openclaw",
						"instanceId": "test-instance-001",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "openclaw",
							Image: "openclaw/openclaw:latest",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "openclaw",
							Ready: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			By("Reconciling to update status")
			controllerReconciler := &OpenClawInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking instance status")
			Eventually(func() string {
				instance := &openclawv1alpha1.OpenClawInstance{}
				err := k8sClient.Get(ctx, typeNamespacedName, instance)
				if err != nil {
					return ""
				}
				return instance.Status.Phase
			}, time.Minute, time.Second).Should(Equal(openclawv1alpha1.PhaseRunning))
		})

		It("should handle resource deletion", func() {
			By("Adding deletion timestamp")
			resource := &openclawv1alpha1.OpenClawInstance{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			now := metav1.Now()
			resource.DeletionTimestamp = &now
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())

			By("Reconciling the deletion")
			controllerReconciler := &OpenClawInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating Deployment", func() {
		It("should create correct Deployment spec", func() {
			instance := &openclawv1alpha1.OpenClawInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawInstanceSpec{
					InstanceID: "deploy-test-001",
					UserID:     "user-789",
					Image:      "openclaw/openclaw:v1.0.0",
					Plan:       "pro",
					Resources: openclawv1alpha1.ResourceRequirements{
						CPU:         "500m",
						Memory:      "1Gi",
						CPULimit:    "2000m",
						MemoryLimit: "4Gi",
					},
				},
			}

			reconciler := &OpenClawInstanceReconciler{}
			deployment, err := reconciler.createDeployment(instance)

			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.Name).To(Equal("openclaw-deploy-test-001"))
			Expect(deployment.Namespace).To(Equal("default"))
			Expect(deployment.Labels["app"]).To(Equal("openclaw"))
			Expect(deployment.Labels["instanceId"]).To(Equal("deploy-test-001"))
			Expect(deployment.Labels["userId"]).To(Equal("user-789"))
		})

		It("should set correct hostPort", func() {
			instance := &openclawv1alpha1.OpenClawInstance{
				Spec: openclawv1alpha1.OpenClawInstanceSpec{
					InstanceID: "port-test",
					UserID:     "user-001",
				},
			}

			reconciler := &OpenClawInstanceReconciler{}
			deployment, err := reconciler.createDeployment(instance)

			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(18789)))
		})
	})
})

// Helper function
func int32Ptr(i int32) *int32 {
	return &i
}
