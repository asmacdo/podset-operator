/*
Copyright 2022.

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

package controllers

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	podsetv1alpha1 "github.com/asmacdo/podset-operator/api/v1alpha1"
)

// PodSetReconciler reconciles a PodSet object
type PodSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=podset.example.com,resources=podsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=podset.example.com,resources=podsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=podset.example.com,resources=podsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *PodSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the PodSet instance
	instance := &podsetv1alpha1.PodSet{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// if not found maybe its been deleted, dont requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object, requeue
		return ctrl.Result{}, err
	}

	// LIst all pods owned by this PodSet instance,
	podSet := instance
	podList := &corev1.PodList{}
	podSetLabels := map[string]string{
		"app":     podSet.Name,
		"version": "v0.1",
	}

	labelSelector := labels.SelectorFromSet(podSetLabels)
	listOpts := &client.ListOptions{Namespace: podSet.Namespace, LabelSelector: labelSelector}
	if err = r.List(context.TODO(), podList, listOpts); err != nil {
		return ctrl.Result{}, err
	}
	// Count available pods (running + pending)
	var available []corev1.Pod
	for _, pod := range podList.Items {
		// Dont count deleted pods
		if pod.ObjectMeta.DeletionTimestamp != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			available = append(available, pod)
		}
	}
	numAvailable := int32(len(available))
	availableNames := []string{}
	for _, pod := range available {
		availableNames = append(availableNames, pod.ObjectMeta.Name)
	}

	// Update the status if necessary
	status := podsetv1alpha1.PodSetStatus{
		PodNames:          availableNames,
		AvailableReplicas: numAvailable,
	}
	if !reflect.DeepEqual(podSet.Status, status) {
		podSet.Status = status
		err = r.Status().Update(context.TODO(), podSet)
		if err != nil {
			log.Error(err, "Failed to update PodSet status")
			return ctrl.Result{}, err
		}
	}

	if numAvailable > podSet.Spec.Replicas {
		diff := numAvailable - podSet.Spec.Replicas
		// TODO(asmacdo) should be random?
		dpods := available[:diff]
		for _, dpod := range dpods {
			err = r.Delete(context.TODO(), &dpod)
			if err != nil {
				log.Error(err, "Failed to delete pod", "pod.name", dpod.Name)
				// requeue
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}
	if numAvailable < podSet.Spec.Replicas {
		log.Info("Scaling up pods", "Currently available", numAvailable, "Required replicas", podSet.Spec.Replicas)
		// Define a new Pod Object
		pod := newPodForCR(podSet)
		// Set PodSet instance as the owner and controller
		if err := controllerutil.SetControllerReference(podSet, pod, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		err = r.Create(context.TODO(), pod)
		if err != nil {
			log.Error(err, "Failed to create pod", "pod.name", pod.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func newPodForCR(cr *podsetv1alpha1.PodSet) *corev1.Pod {
	podSetLabels := map[string]string{
		"app":     cr.Name,
		"version": "v0.1",
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-pod",
			Namespace:    cr.Namespace,
			Labels:       podSetLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podsetv1alpha1.PodSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
