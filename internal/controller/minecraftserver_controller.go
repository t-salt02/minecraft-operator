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
		logger.Info("Unable to fetch MinecraftServer's CR", "fetchError", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	headlessSvcName := mc.Name + "-headless"
	var svc corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: headlessSvcName, Namespace: mc.Namespace}, &svc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Headless service not found, creating a new one", "service", headlessSvcName)
			// definetion of headless service
			svc = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      headlessSvcName,
					Namespace: mc.Namespace,
					Labels:    map[string]string{"app": mc.Name},
					Annotations: map[string]string{
						"tailscale.com/expose":   "true",
						"tailscale.com/hostname": mc.Spec.ServerName,
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       25565,
							Protocol:   corev1.ProtocolTCP,
							TargetPort: intstr.FromString("minecraft"),
						},
					},
					ClusterIP: "None",
					Selector:  map[string]string{"app": mc.Name},
				},
			}
			if err := ctrl.SetControllerReference(&mc, &svc, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, &svc); err != nil {
				logger.Error(err, "Unable to create headless service")
				return ctrl.Result{}, err
			}
			logger.Info("Created headless service", "service", svc.Name)
		} else {
			logger.Error(err, "Unable to fetch headless service")
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("Headless service already exists", "service", svc.Name)
	}

	// Create or update the StatefulSet
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, &sts); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StatefulSet not found, creating a new one", "statefulset", mc.Name)
			sts = appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mc.Name,
					Namespace: mc.Namespace,
					Labels:    map[string]string{"app": mc.Name},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas:    ptr.To(int32(1)),
					ServiceName: headlessSvcName,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": mc.Name},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": mc.Name},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
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
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
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
					},
				},
			}
			if err := ctrl.SetControllerReference(&mc, &sts, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, &sts); err != nil {
				logger.Error(err, "Unable to create StatefulSet")
				return ctrl.Result{}, err
			}
			logger.Info("Created StatefulSet", "statefulset", sts.Name)
		} else {
			return ctrl.Result{}, err
		}
	}
	// update statefulset status
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(mc.Namespace), client.MatchingLabels{"app": mc.Name}); err != nil {
		logger.Error(err, "Unable to list pods")
		return ctrl.Result{}, err
	}
	if len(podList.Items) == 0 {
		logger.Info("No pods found for StatefulSet", "statefulset", mc.Name)
		mc.Status.Ready = false
		mc.Status.IP = ""
	} else {
		pod := podList.Items[0]
		mc.Status.Ready = len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready
		if mc.Status.Ready {
			mc.Status.IP = pod.Status.PodIP
		} else {
			mc.Status.IP = ""
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}
	// update status
	if err := r.Status().Update(ctx, &mc); err != nil {
		return ctrl.Result{}, err
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
