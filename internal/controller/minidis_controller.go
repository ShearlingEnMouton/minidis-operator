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
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/recorder"

	minidisv1alpha1 "github.com/ShearlingEnMouton/minidis-operator/api/v1alpha1"
)

// MinidisReconciler reconciles a Minidis object
type MinidisReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder recorder.EventRecorder
}

// +kubebuilder:rbac:groups=minidis.shearlingenmouton.github.io,resources=minidis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minidis.shearlingenmouton.github.io,resources=minidis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minidis.shearlingenmouton.github.io,resources=minidis/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func minidisLabel(m minidisv1alpha1.Minidis) map[string]string {
	return map[string]string{
		"app": m.Name,
	}
}

// readyCondition builds the Ready condition of m with the given status, reason and message.
func readyCondition(m *minidisv1alpha1.Minidis, status v1.ConditionStatus, reason, message string) v1.Condition {
	return v1.Condition{
		Type:               "Ready",
		Status:             status,
		ObservedGeneration: m.Generation,
		Reason:             reason,
		Message:            message,
	}
}

// reportFailure records a failed reconcile of the named object as a Warning event and
// as a False Ready condition, and returns err so that the request is retried.
func (r *MinidisReconciler) reportFailure(ctx context.Context, m *minidisv1alpha1.Minidis, object string, err error) error {
	r.Recorder.Eventf(m, nil, corev1.EventTypeWarning, "ReconcileFailed", "CreateOrUpdate", "failed to reconcile %s: %v", object, err)

	meta.SetStatusCondition(&m.Status.Conditions,
		readyCondition(m, v1.ConditionFalse, "ReconcileFailed", fmt.Sprintf("failed to reconcile %s: %v", object, err)))
	if updateErr := r.Status().Update(ctx, m); updateErr != nil {
		return errors.Join(err, updateErr)
	}
	return err
}

// Reconcile brings the resources of a Minidis object in line with its spec.
func (r *MinidisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)
	var err error

	m := minidisv1alpha1.Minidis{}
	if err := r.Get(ctx, req.NamespacedName, &m); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	service := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := controllerutil.SetControllerReference(&m, service, r.Scheme); err != nil {
			return err
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = minidisLabel(m)
		service.Spec.Ports = []corev1.ServicePort{
			{
				Protocol:   corev1.ProtocolTCP,
				Port:       3131,
				TargetPort: intstr.FromInt32(3131),
			},
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, r.reportFailure(ctx, &m, "Service", err)
	}

	headless := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      m.Name + "-headless",
			Namespace: m.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, headless, func() error {
		if err := controllerutil.SetControllerReference(&m, headless, r.Scheme); err != nil {
			return err
		}
		if headless.CreationTimestamp.IsZero() {
			headless.Spec.ClusterIP = corev1.ClusterIPNone
		}
		headless.Spec.Selector = minidisLabel(m)
		headless.Spec.Ports = []corev1.ServicePort{
			{
				Protocol:   corev1.ProtocolTCP,
				Port:       3131,
				TargetPort: intstr.FromInt32(3131),
			},
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, r.reportFailure(ctx, &m, "headless Service", err)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
	}
	stsEnv := []corev1.EnvVar{
		{
			Name:  "EXPOSE_PORT",
			Value: "3131",
		},
		{
			Name:  "BIND_ADDR",
			Value: "0.0.0.0",
		},
		{
			Name:  "AOF_PATH",
			Value: "/data/appendonly.aof",
		},
		{
			Name:  "AOF_FSYNC",
			Value: m.Spec.Fsync,
		},
	}
	if m.Spec.Auth != nil {
		stsEnv = append(stsEnv, corev1.EnvVar{
			Name: "REQUIREPASS",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: m.Spec.Auth,
			},
		})
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		if err := controllerutil.SetControllerReference(&m, sts, r.Scheme); err != nil {
			return err
		}
		if sts.CreationTimestamp.IsZero() {
			sts.Spec.Selector = &v1.LabelSelector{
				MatchLabels: minidisLabel(m),
			}
			sts.Spec.ServiceName = headless.Name
			sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "minidis-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: m.Spec.Storage.StorageClassName,
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: m.Spec.Storage.Size,
							},
						},
					},
				},
			}
		}
		sts.Spec.Replicas = new(int32(1))
		sts.Spec.Template.Labels = minidisLabel(m)

		if len(sts.Spec.Template.Spec.Containers) == 0 {
			sts.Spec.Template.Spec.Containers = []corev1.Container{{Name: "minidis"}}
		}
		c := &sts.Spec.Template.Spec.Containers[0]
		c.Image = "ghcr.io/shearlingenmouton/minidis:" + m.Spec.Version
		c.ImagePullPolicy = corev1.PullIfNotPresent
		c.Ports = []corev1.ContainerPort{
			{
				ContainerPort: 3131,
				Protocol:      corev1.ProtocolTCP,
			},
		}
		c.Env = stsEnv
		c.Resources = m.Spec.Resources
		c.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "minidis-data",
				MountPath: "/data",
			},
		}
		healthcheck := corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"/usr/local/bin/minidis", "healthcheck"},
			},
		}
		c.StartupProbe = &corev1.Probe{
			ProbeHandler:     healthcheck,
			PeriodSeconds:    5,
			TimeoutSeconds:   5,
			SuccessThreshold: 1,
			FailureThreshold: 30,
		}
		c.LivenessProbe = &corev1.Probe{
			ProbeHandler:     healthcheck,
			PeriodSeconds:    5,
			TimeoutSeconds:   10,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		}
		c.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(3131),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      1,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, r.reportFailure(ctx, &m, "StatefulSet", err)
	}

	condition := readyCondition(&m, v1.ConditionFalse, "NotReady", "minidis pod is not ready")
	if sts.Status.ReadyReplicas == 1 {
		condition = readyCondition(&m, v1.ConditionTrue, "Ready", "minidis pod is ready")
	}
	m.Status.ReadyReplicas = sts.Status.ReadyReplicas
	m.Status.ObservedGeneration = m.Generation
	meta.SetStatusCondition(&m.Status.Conditions, condition)
	if err := r.Status().Update(ctx, &m); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinidisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minidisv1alpha1.Minidis{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Named("minidis").
		Complete(r)
}
