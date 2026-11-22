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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	minidisv1alpha1 "github.com/ShearlingEnMouton/minidis-operator/api/v1alpha1"
)

const testNamespace = "default"

func envValue(c corev1.Container, name string) (corev1.EnvVar, bool) {
	for _, e := range c.Env {
		if e.Name == name {
			return e, true
		}
	}
	return corev1.EnvVar{}, false
}

var _ = Describe("Minidis Controller", func() {
	ctx := context.Background()
	var reconciler *MinidisReconciler
	var recorder *events.FakeRecorder

	BeforeEach(func() {
		recorder = events.NewFakeRecorder(10)
		reconciler = &MinidisReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: recorder,
		}
	})

	create := func(name string, spec minidisv1alpha1.MinidisSpec) types.NamespacedName {
		m := &minidisv1alpha1.Minidis{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, m)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, m)).To(Succeed())
		})
		return types.NamespacedName{Name: name, Namespace: testNamespace}
	}

	reconcileOnce := func(key types.NamespacedName) {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
	}

	It("creates the Services and the StatefulSet from the defaults", func() {
		key := create("defaults", minidisv1alpha1.MinidisSpec{})
		reconcileOnce(key)

		m := &minidisv1alpha1.Minidis{}
		Expect(k8sClient.Get(ctx, key, m)).To(Succeed())

		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, key, svc)).To(Succeed())
		Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		Expect(svc.Spec.Selector).To(Equal(minidisLabel(*m)))
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Port).To(BeEquivalentTo(3131))
		Expect(metav1.IsControlledBy(svc, m)).To(BeTrue())

		headless := &corev1.Service{}
		headlessKey := types.NamespacedName{Name: key.Name + "-headless", Namespace: testNamespace}
		Expect(k8sClient.Get(ctx, headlessKey, headless)).To(Succeed())
		Expect(headless.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
		Expect(headless.Spec.Selector).To(Equal(minidisLabel(*m)))
		Expect(metav1.IsControlledBy(headless, m)).To(BeTrue())

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, sts)).To(Succeed())
		Expect(metav1.IsControlledBy(sts, m)).To(BeTrue())
		Expect(*sts.Spec.Replicas).To(BeEquivalentTo(1))
		Expect(sts.Spec.ServiceName).To(Equal(headlessKey.Name))
		Expect(sts.Spec.Selector.MatchLabels).To(Equal(minidisLabel(*m)))
		Expect(sts.Spec.Template.Labels).To(Equal(minidisLabel(*m)))

		Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
		c := sts.Spec.Template.Spec.Containers[0]
		Expect(c.Image).To(Equal("ghcr.io/shearlingenmouton/minidis:1.0.0"))
		fsync, ok := envValue(c, "AOF_FSYNC")
		Expect(ok).To(BeTrue())
		Expect(fsync.Value).To(Equal("everysec"))
		_, ok = envValue(c, "REQUIREPASS")
		Expect(ok).To(BeFalse())
		Expect(c.VolumeMounts).To(ContainElement(HaveField("MountPath", "/data")))

		Expect(sts.Spec.VolumeClaimTemplates).To(HaveLen(1))
		pvc := sts.Spec.VolumeClaimTemplates[0]
		Expect(pvc.Name).To(Equal("minidis-data"))
		Expect(pvc.Spec.StorageClassName).To(BeNil())
		size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		Expect(size.Cmp(resource.MustParse("2Gi"))).To(Equal(0))
	})

	It("applies the spec fields to the StatefulSet", func() {
		className := "fast"
		key := create("custom", minidisv1alpha1.MinidisSpec{
			Version: "0.9.0",
			Fsync:   "always",
			Storage: minidisv1alpha1.MinidisStorage{
				Size:             resource.MustParse("5Gi"),
				StorageClassName: &className,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
			},
			Auth: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "minidis-auth"},
				Key:                  "password",
			},
		})
		reconcileOnce(key)

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, sts)).To(Succeed())
		c := sts.Spec.Template.Spec.Containers[0]
		Expect(c.Image).To(Equal("ghcr.io/shearlingenmouton/minidis:0.9.0"))
		Expect(c.Resources.Requests.Memory().Cmp(resource.MustParse("64Mi"))).To(Equal(0))

		fsync, _ := envValue(c, "AOF_FSYNC")
		Expect(fsync.Value).To(Equal("always"))
		pass, ok := envValue(c, "REQUIREPASS")
		Expect(ok).To(BeTrue())
		Expect(pass.ValueFrom.SecretKeyRef.Name).To(Equal("minidis-auth"))
		Expect(pass.ValueFrom.SecretKeyRef.Key).To(Equal("password"))

		pvc := sts.Spec.VolumeClaimTemplates[0]
		Expect(pvc.Spec.StorageClassName).To(HaveValue(Equal("fast")))
		size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		Expect(size.Cmp(resource.MustParse("5Gi"))).To(Equal(0))
	})

	It("reports Ready as False until the pod is ready", func() {
		key := create("status", minidisv1alpha1.MinidisSpec{})
		reconcileOnce(key)

		m := &minidisv1alpha1.Minidis{}
		Expect(k8sClient.Get(ctx, key, m)).To(Succeed())
		Expect(m.Status.ReadyReplicas).To(BeEquivalentTo(0))
		Expect(m.Status.ObservedGeneration).To(Equal(m.Generation))
		cond := meta.FindStatusCondition(m.Status.Conditions, "Ready")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal("NotReady"))
	})

	It("reports Ready as True once the StatefulSet has a ready replica", func() {
		key := create("ready", minidisv1alpha1.MinidisSpec{})
		reconcileOnce(key)

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, sts)).To(Succeed())
		sts.Status.Replicas = 1
		sts.Status.ReadyReplicas = 1
		Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
		reconcileOnce(key)

		m := &minidisv1alpha1.Minidis{}
		Expect(k8sClient.Get(ctx, key, m)).To(Succeed())
		Expect(m.Status.ReadyReplicas).To(BeEquivalentTo(1))
		cond := meta.FindStatusCondition(m.Status.Conditions, "Ready")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	})

	It("does not update the StatefulSet when nothing changed", func() {
		key := create("idempotent", minidisv1alpha1.MinidisSpec{})
		reconcileOnce(key)

		before := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, before)).To(Succeed())
		reconcileOnce(key)

		after := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, after)).To(Succeed())
		Expect(after.ResourceVersion).To(Equal(before.ResourceVersion))
	})

	It("updates the image when the version changes", func() {
		key := create("upgrade", minidisv1alpha1.MinidisSpec{})
		reconcileOnce(key)

		m := &minidisv1alpha1.Minidis{}
		Expect(k8sClient.Get(ctx, key, m)).To(Succeed())
		m.Spec.Version = "1.1.0"
		Expect(k8sClient.Update(ctx, m)).To(Succeed())
		reconcileOnce(key)

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/shearlingenmouton/minidis:1.1.0"))
	})

	It("reports a failed reconcile in the Ready condition and as an event", func() {
		key := create("conflict", minidisv1alpha1.MinidisSpec{})

		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "conflict-owner", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(ctx, owner)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, owner)).To(Succeed())
		})

		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: testNamespace},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 3131}},
			},
		}
		Expect(controllerutil.SetControllerReference(owner, svc, k8sClient.Scheme())).To(Succeed())
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, svc)).To(Succeed())
		})

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).To(HaveOccurred())

		m := &minidisv1alpha1.Minidis{}
		Expect(k8sClient.Get(ctx, key, m)).To(Succeed())
		cond := meta.FindStatusCondition(m.Status.Conditions, "Ready")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal("ReconcileFailed"))
		Expect(cond.Message).To(ContainSubstring("failed to reconcile Service"))

		Expect(recorder.Events).To(Receive(HavePrefix("Warning ReconcileFailed failed to reconcile Service")))
	})

	It("ignores a Minidis that no longer exists", func() {
		reconcileOnce(types.NamespacedName{Name: "missing", Namespace: testNamespace})
	})
})
