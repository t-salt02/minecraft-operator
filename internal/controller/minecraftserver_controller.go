/*
Copyright 2025.

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
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	minecraftv1alpha1 "github.com/t-salt02/minecraft-operator/api/v1alpha1"
)

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=minecraft.mcop.co-salt.com,resources=minecraftservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minecraft.mcop.co-salt.com,resources=minecraftservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minecraft.mcop.co-salt.com,resources=minecraftservers/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *MinecraftServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Reconciling MinecraftServer")

	var mc minecraftv1alpha1.MinecraftServer
	if err := r.Get(ctx, req.NamespacedName, &mc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("MinecraftServer resource not found.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Unable to fetch MinecraftServer's CR", "fetchError", mc.Name)
		return ctrl.Result{}, err
	}

	headlessSvcName := mc.Name + "-headless"
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      headlessSvcName,
			Namespace: mc.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		svc.SetLabels(map[string]string{"app": mc.Name})
		svc.SetAnnotations(map[string]string{
			"tailscale.com/expose":   "true",
			"tailscale.com/hostname": mc.Spec.ServerName,
		})
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Port:       25565,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("minecraft"),
			},
		}
		svc.Spec.ClusterIP = "None"
		svc.Spec.Selector = map[string]string{"app": mc.Name}
		return ctrl.SetControllerReference(&mc, &svc, r.Scheme)
	})
	if err != nil {
		logger.Error(err, "Unable to create or update headless service")
		return ctrl.Result{}, err
	}
	logger.Info("Headless service created or updated", "service", svc.Name)

	// Create or update the StatefulSet
	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc.Name,
			Namespace: mc.Namespace,
		},
	}

	_, err = ctrl.CreateOrUpdate(ctx, r.Client, &sts, func() error {
		sts.SetLabels(map[string]string{"app": mc.Name})
		sts.Spec.Replicas = ptr.To(int32(1))
		sts.Spec.ServiceName = headlessSvcName
		sts.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": mc.Name},
		}
		sts.Spec.Template.ObjectMeta.Labels = map[string]string{"app": mc.Name}
		sts.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  mc.Name,
				Image: "itzg/minecraft-server",
				Env: []corev1.EnvVar{
					{Name: "EULA", Value: "TRUE"},
					{Name: "DIFFICULTY", Value: mc.Spec.Difficulty},
					{Name: "SEED", Value: mc.Spec.Seed},
					{Name: "HARDCORE", Value: strconv.FormatBool(mc.Spec.Hardcore)},
					{Name: "SERVER_NAME", Value: mc.Spec.ServerName},
					{Name: "VERSION", Value: mc.Spec.Version},
					{Name: "MEMORY", Value: "4G"},
				},
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 25565,
						Name:          "minecraft",
						Protocol:      corev1.ProtocolTCP,
					},
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2000m"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			},
		}
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   mc.Name,
					Labels: map[string]string{"app": mc.Name},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(mc.Spec.Storage),
						},
					},
					StorageClassName: ptr.To("topolvm-provisioner"),
				},
			},
		}
		return ctrl.SetControllerReference(&mc, &sts, r.Scheme)
	})
	if err != nil {
		logger.Error(err, "Unable to create or update StatefulSet")
		return ctrl.Result{}, err
	}
	logger.Info("StatefulSet created or updated", "statefulset", sts.Name)

	//update statefulset status
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(mc.Namespace), client.MatchingLabels{"app": mc.Name}); err != nil {
		logger.Error(err, "Unable to list pods for statefulSet", "statefulset", mc.Name)
		mc.Status.Ready = false
		mc.Status.IP = ""
		if err := r.Status().Update(ctx, &mc); err != nil {
			logger.Error(err, "Unable to update MinecraftServer status", "updateError", err)
			return ctrl.Result{}, err
		}
	}
	if len(podList.Items) == 0 {
		logger.Info("No pods found for StatefulSet", "statefulset", mc.Name)
		mc.Status.Ready = false
		mc.Status.IP = ""
		if err := r.Status().Update(ctx, &mc); err != nil {
			logger.Error(err, "Unable to update MinecraftServer status", "updateError", err)
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Check if the pod is ready
	pod := podList.Items[0]
	isPodReady := false
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
		isPodReady = true
	}

	// Check if the service is created
	isServiceReady := false
	if err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &svc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Service not found", "service", svc.Name)
		} else {
			logger.Error(err, "Unable to fetch service", "service", svc.Name)
		}
	} else {
		isServiceReady = true
	}

	if isPodReady && isServiceReady {
		mc.Status.Ready = true
		mc.Status.IP = pod.Status.PodIP
	} else {
		mc.Status.Ready = false
		mc.Status.IP = ""
	}

	if err := r.Status().Update(ctx, &mc); err != nil {
		logger.Error(err, "Unable to update MinecraftServer status", "updateError", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minecraftv1alpha1.MinecraftServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Pod{}).
		Named("minecraftserver").
		Complete(r)
}
