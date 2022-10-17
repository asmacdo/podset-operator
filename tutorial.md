# Go Operator - PodSet

## Prerequisites

kind, minikube or some cluster

`kubectl cluster-info`

```
Kubernetes control plane is running at https://127.0.0.1:41829
CoreDNS is running at https://127.0.0.1:41829/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy

To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'
```

## Initialize the project

Let’s now create a new directory for our project:

`mkdir -p $HOME/projects/podset-operator`

Navigate to the directory:

`cd $HOME/projects/podset-operator`

Initialize a new Go-based Operator SDK project for the PodSet Operator:

`operator-sdk init --domain=example.com --repo=github.com/redhat/podset-operator`


## Create API for Custom Resource

`operator-sdk create api --group=app --version=v1alpha1 --kind=PodSet --resource --controller`

We should now see the api, config, and controllers directories.Let’s
begin by inspecting the newly generated api/v1alpha1/podset_types.go
file for our PodSet API:

`cat api/v1alpha1/podset_types.go`

In Kubernetes, every functional object (with some exceptions, i.e.
ConfigMap) includes spec and status. Kubernetes functions by reconciling
desired state (Spec) with the actual cluster state. We then record what
is observed (Status).

Also observe the +kubebuilder comment markers found throughout the file.
operator-sdk makes use of a tool called controler-gen (from the
controller-tools project) for generating utility code and Kubernetes
YAML. More information on markers for config/code generation can be
found here.

Let’s now modify the PodSetSpec and PodSetStatus of the PodSet Custom
Resource (CR) at api/v1alpha1/podset_types.go

It should look like the file below:

```
package v1alpha1

import (
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PodSetSpec defines the desired state of PodSet
type PodSetSpec struct {
        // Replicas is the desired number of pods for the PodSet
        // +kubebuilder:validation:Minimum=1
        // +kubebuilder:validation:Maximum=10
        Replicas int32 `json:"replicas,omitempty"`
}

// PodSetStatus defines the current status of PodSet
type PodSetStatus struct {
        PodNames        []string        `json:"podNames"`
	AvailableReplicas	int32	`json:"availableReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PodSet is the Schema for the podsets API
// +kubebuilder:printcolumn:JSONPath=".spec.replicas",name=Desired,type=string
// +kubebuilder:printcolumn:JSONPath=".status.availableReplicas",name=Available,type=string
type PodSet struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   PodSetSpec   `json:"spec,omitempty"`
        Status PodSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PodSetList contains a list of PodSet
type PodSetList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []PodSet `json:"items"`
}

func init() {
        SchemeBuilder.Register(&PodSet{}, &PodSetList{})
}
```

You can easily update this file by running the following command:

`\cp /tmp/podset_types.go api/v1alpha1/podset_types.go`

After modifying the `*_types.go` file, always run the following command to update the zz_generated.deepcopy.go file:

`make generate`

Now we can run the make manifests command to generate our customized CRD and additional object YAMLs.

`make manifests`

Thanks to our comment markers, observe that we now have a newly generated CRD yaml that reflects the spec.replicas and status.podNames OpenAPI v3 schema validation and customized print columns.

`cat config/crd/bases/app.example.com_podsets.yaml`

Deploy your PodSet Custom Resource Definition to the live OpenShift Cluster:

`kubectl apply -f config/crd/bases/app.example.com_podsets.yaml`

Confirm the CRD was successfully created:

`kubectl get crd podsets.app.example.com -o yaml`

Let’s now observe the default controllers/podset_controller.go file:

`cat controllers/podset_controller.go`

This default controller requires additional logic so we can trigger our reconciler whenever kind: PodSet objects are added, updated, or deleted. We also want to trigger the reconciler whenever Pods owned by a given PodSet are added, updated, and deleted as well. To accomplish this. we modify the controller’s SetupWithManager method.

Modify the PodSet controller logic at controllers/podset_controller.go:

```
/*
Copyright 2021.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/redhat/podset-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// PodSetReconciler reconciles a PodSet object
type PodSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=app.example.com,resources=podsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.example.com,resources=podsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.example.com,resources=podsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *PodSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the PodSet instance
	instance := &appv1alpha1.PodSet{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err

	}

	// List all pods owned by this PodSet instance
	podSet := instance
	podList := &corev1.PodList{}
	lbs := map[string]string{
		"app":     podSet.Name,
		"version": "v0.1",
	}
	labelSelector := labels.SelectorFromSet(lbs)
	listOps := &client.ListOptions{Namespace: podSet.Namespace, LabelSelector: labelSelector}
	if err = r.List(context.TODO(), podList, listOps); err != nil {
		return ctrl.Result{}, err
	}

	// Count the pods that are pending or running as available
	var available []corev1.Pod
	for _, pod := range podList.Items {
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
	status := appv1alpha1.PodSetStatus{
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
		log.Info("Scaling down pods", "Currently available", numAvailable, "Required replicas", podSet.Spec.Replicas)
		diff := numAvailable - podSet.Spec.Replicas
		dpods := available[:diff]
		for _, dpod := range dpods {
			err = r.Delete(context.TODO(), &dpod)
			if err != nil {
				log.Error(err, "Failed to delete pod", "pod.name", dpod.Name)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if numAvailable < podSet.Spec.Replicas {
		log.Info("Scaling up pods", "Currently available", numAvailable, "Required replicas", podSet.Spec.Replicas)
		// Define a new Pod object
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

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *appv1alpha1.PodSet) *corev1.Pod {
	labels := map[string]string{
		"app":     cr.Name,
		"version": "v0.1",
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-pod",
			Namespace:    cr.Namespace,
			Labels:       labels,
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
		For(&appv1alpha1.PodSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

```

You can easily update this file by running the following command:

`\cp /tmp/podset_controller.go controllers/podset_controller.go`

go mod tidy ensures that the go.mod file matches the source code in the module. It adds any missing module requirements necessary to build the current module’s packages and dependencies, and it removes requirements on modules that don’t provide any relevant packages. It also adds any missing entries to go.sum and removes unnecessary entries.

`go mod tidy`

Once the CRD is registered, there are two ways to run the Operator:

- As a Pod inside a Kubernetes cluster
- As a Go program outside the cluster using Operator-SDK. This is great for local development of your Operator.

For the sake of this tutorial, we will run the Operator as a Go program outside the cluster using Operator-SDK and our kubeconfig credentials

Once running, the command will block the current session. You can continue interacting with the OpenShift cluster by opening a new terminal window. You can quit the session by pressing CTRL + C.

<!-- WATCH_NAMESPACE=myproject make run -->
`make run`

In your new terminal, run the following:

`cd $HOME/projects/podset-operator`
`cat config/samples/app_v1alpha1_podset.yaml`

Ensure your kind: PodSet Custom Resource (CR) is updated with spec.replicas

```
apiVersion: app.example.com/v1alpha1
kind: PodSet
metadata:
  name: podset-sample
spec:
  replicas: 3
```

You can easily update this file by running the following command:

`\cp /tmp/app_v1alpha1_podset.yaml config/samples/app_v1alpha1_podset.yaml`

<!-- Ensure you are currently scoped to the myproject Namespace: -->
<!-- Can we get away with just default? -->

Deploy your PodSet Custom Resource to the live Kubernetes cluster:

`kubectl apply -f config/samples/app_v1alpha1_podset.yaml`

Verify the Podset exists:

`kubectl get podsets`

Verify the PodSet operator has created 3 pods:

`kubectl get pods`

Verify that status shows the name of the pods currently owned by the PodSet:

`kubectl get podset podset-sample -o yaml`

Increase the number of replicas owned by the PodSet:

`kubectl patch podset podset-sample --type='json' -p '[{"op": "replace", "path": "/spec/replicas", "value":5}]'`

Verify that we now have 5 running pods

`kubectl get pods`

Our PodSet controller creates pods containing OwnerReferences in their metadata section. This ensures they will be removed upon deletion of the podset-sample CR.

Observe the OwnerReference set on a Podset’s pod:

`kubectl get pods -o yaml | grep ownerReferences -A10`

Delete the podset-sample Custom Resource:

`kubectl delete podset podset-sample`

Thanks to OwnerReferences, all of the pods should be deleted:

`kubectl get pods`
