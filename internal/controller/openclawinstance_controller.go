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
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawv1alpha1 "github.com/openclaw/operator/api/v1alpha1"
)

const (
	// FinalizerName is the finalizer used by this controller
	FinalizerName = "openclaw.platform/finalizer"

	// RequeueAfter is the default requeue interval
	RequeueAfter = 30 * time.Second

	// HealthCheckInterval is how often to check pod health
	HealthCheckInterval = 10 * time.Second
)

// OpenClawInstanceReconciler reconciles a OpenClawInstance object
type OpenClawInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=openclaw.platform,resources=openclawinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openclaw.platform,resources=openclawinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openclaw.platform,resources=openclawinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *OpenClawInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling OpenClawInstance", "name", req.Name)

	// Fetch the OpenClawInstance
	instance := &openclawv1alpha1.OpenClawInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("OpenClawInstance not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get OpenClawInstance")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(instance, FinalizerName) {
		logger.Info("Adding finalizer")
		controllerutil.AddFinalizer(instance, FinalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial phase if not set
	if instance.Status.Phase == "" {
		instance.Status.Phase = openclawv1alpha1.PhasePending
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile all resources
	if err := r.reconcileResources(ctx, instance); err != nil {
		logger.Error(err, "Failed to reconcile resources")
		r.Recorder.Event(instance, corev1.EventTypeWarning, "ReconcileFailed", err.Error())

		// Update status to Error
		instance.Status.Phase = openclawv1alpha1.PhaseError
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    openclawv1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ReconcileFailed",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	// Check health and update status
	if err := r.reconcileHealth(ctx, instance); err != nil {
		logger.Error(err, "Health check failed")
		return ctrl.Result{RequeueAfter: HealthCheckInterval}, nil
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// reconcileResources creates/updates all managed resources
func (r *OpenClawInstanceReconciler) reconcileResources(ctx context.Context, instance *openclawv1alpha1.OpenClawInstance) error {
	logger := log.FromContext(ctx)

	// Create Deployment
	deployment, err := r.createDeployment(instance)
	if err != nil {
		return fmt.Errorf("failed to create deployment spec: %w", err)
	}

	if err := controllerutil.SetControllerReference(instance, deployment, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	existingDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: deployment.Name}, existingDeployment)
	if apierrors.IsNotFound(err) {
		logger.Info("Creating Deployment", "name", deployment.Name)
		if err := r.Create(ctx, deployment); err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}
		r.Recorder.Event(instance, corev1.EventTypeNormal, "DeploymentCreated", "Deployment created")
	} else if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	} else {
		// Update if needed
		if needsUpdate(existingDeployment, deployment) {
			logger.Info("Updating Deployment", "name", deployment.Name)
			existingDeployment.Spec = deployment.Spec
			if err := r.Update(ctx, existingDeployment); err != nil {
				return fmt.Errorf("failed to update deployment: %w", err)
			}
			r.Recorder.Event(instance, corev1.EventTypeNormal, "DeploymentUpdated", "Deployment updated")
		}
	}

	// Update phase to Running
	if instance.Status.Phase != openclawv1alpha1.PhaseRunning {
		instance.Status.Phase = openclawv1alpha1.PhaseCreating
		if err := r.Status().Update(ctx, instance); err != nil {
			return err
		}
	}

	return nil
}

// reconcileHealth checks pod health and updates status
func (r *OpenClawInstanceReconciler) reconcileHealth(ctx context.Context, instance *openclawv1alpha1.OpenClawInstance) error {
	logger := log.FromContext(ctx)

	// Find the Pod
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(instance.Namespace), client.MatchingLabels{
		"app":        "openclaw",
		"instanceId": instance.Spec.InstanceID,
	}); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found yet")
		return nil
	}

	pod := podList.Items[0]
	instance.Status.PodName = pod.Name
	instance.Status.HostPort = 18789

	// Get node IP
	if pod.Spec.NodeName != "" {
		node := &corev1.Node{}
		if err := r.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node); err == nil {
			for _, addr := range node.Status.Addresses {
				if addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP {
					instance.Status.HostIP = addr.Address
					instance.Status.GatewayEndpoint = fmt.Sprintf("http://%s:18789", addr.Address)
					break
				}
			}
		}
	}

	// Check pod phase
	switch pod.Status.Phase {
	case corev1.PodRunning:
		// Check if containers are ready
		ready := true
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready {
				ready = false
				break
			}
		}

		if ready {
			instance.Status.Phase = openclawv1alpha1.PhaseRunning
			instance.Status.LastHealthCheck = &metav1.Time{Time: time.Now()}
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:   openclawv1alpha1.ConditionTypeReady,
				Status: metav1.ConditionTrue,
				Reason: "PodReady",
			})
			r.Recorder.Event(instance, corev1.EventTypeNormal, "InstanceReady", "Instance is running and healthy")
		} else {
			instance.Status.Phase = openclawv1alpha1.PhaseCreating
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:   openclawv1alpha1.ConditionTypeReady,
				Status: metav1.ConditionFalse,
				Reason: "ContainersNotReady",
			})
		}

	case corev1.PodPending:
		instance.Status.Phase = openclawv1alpha1.PhaseCreating
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   openclawv1alpha1.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "PodPending",
		})

	case corev1.PodFailed:
		instance.Status.Phase = openclawv1alpha1.PhaseError
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   openclawv1alpha1.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "PodFailed",
		})
		r.Recorder.Event(instance, corev1.EventTypeWarning, "InstanceFailed", "Pod failed")

	case corev1.PodUnknown:
		instance.Status.Phase = openclawv1alpha1.PhaseError
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   openclawv1alpha1.ConditionTypeReady,
			Status: metav1.ConditionUnknown,
			Reason: "PodUnknown",
		})
	}

	return r.Status().Update(ctx, instance)
}

// reconcileDelete handles instance deletion
func (r *OpenClawInstanceReconciler) reconcileDelete(ctx context.Context, instance *openclawv1alpha1.OpenClawInstance) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting OpenClawInstance")

	if controllerutil.ContainsFinalizer(instance, FinalizerName) {
		// Clean up external resources here (e.g., notify backend API)

		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, FinalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// createDeployment creates a Deployment spec for the instance
func (r *OpenClawInstanceReconciler) createDeployment(instance *openclawv1alpha1.OpenClawInstance) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("openclaw-%s", instance.Spec.InstanceID),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app":        "openclaw",
				"instanceId": instance.Spec.InstanceID,
				"userId":     instance.Spec.UserID,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        "openclaw",
					"instanceId": instance.Spec.InstanceID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "openclaw",
						"instanceId": instance.Spec.InstanceID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "openclaw",
							Image: instance.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "gateway",
									ContainerPort: 18789,
									HostPort:      18789,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(instance.Spec.Resources.CPU),
									corev1.ResourceMemory: resource.MustParse(instance.Spec.Resources.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(instance.Spec.Resources.CPULimit),
									corev1.ResourceMemory: resource.MustParse(instance.Spec.Resources.MemoryLimit),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/root/.openclaw/workspace",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(18789),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(18789),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: fmt.Sprintf("/data/openclaw/%s", instance.Spec.InstanceID),
									Type: hostPathTypePtr(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
					},
				},
			},
		},
	}

	return deployment, nil
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

func needsUpdate(existing, desired *appsv1.Deployment) bool {
	return !reflect.DeepEqual(existing.Spec.Template.Spec, desired.Spec.Template.Spec)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenClawInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawv1alpha1.OpenClawInstance{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
