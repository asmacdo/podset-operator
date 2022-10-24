# Go Operator - PodSet

## Prerequisites

kind, minikube or some cluster

`kubectl cluster-info`

```
Kubernetes control plane is running at https://127.0.0.1:41829
CoreDNS is running at https://127.0.0.1:41829/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy

To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'
```
## Initialize Project and Create API

First we will create a new project and use the Operator-SDK and
Kubebuilder to scaffold a minimal operator

`mkdir -p $HOME/projects/podset-operator`
`cd $HOME/projects/podset-operator`

Initialize a new Go-based Operator SDK project for the PodSet Operator:

Note: Be sure to substitute your github handle for mhrivnak :)

`operator-sdk init --domain=example.com --repo=github.com/mhrivnak/podset-operator`


## Create API for Custom Resource

Now that we have the skeleton for a project, we need to create our API
in the form of a Kubernetes Custom Resource Definition (CRD), as well as
a controller to interact with that CRD.

`operator-sdk create api --group=app --version=v1alpha1 --kind=PodSet --resource --controller`

We should now see the api, config, and controllers directories.

## Hello World!

Let’s now observe the default `controllers/podset_controller.go` file,
starting with `SetupWithManager`.

```
// SetupWithManager sets up the controller with the Manager.
func (r *PodSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.PodSet{}).
		Complete(r)
}
```

For us, the key line is `For(&appv1alpha1.PodSet{})`, which causes the
reconcile loop to be run each time a PodSet is created, updated, or deleted.

Let's begin by logging "Hello World". Add `	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
` to the imports, and change the `Reconcile` function to:

```
func (r *PodSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Hello World")
	return ctrl.Result{}, nil
}
```

Next, we need to generate the CRD for a PodSet. We'll cover this in a
moment. For now just run:

```
make manifests
```

Make sure the module requirements are installed.

```
go mod tidy
```

Now we are ready to run our most basic operator!

Install the CRD (allowing PodSets to be created later) onto the cluster:

```
make install
```

Now run your operator locally

```
make run
```

You'll see it startup and then you'll see if we ;w were successful in
the logs.

```
1.66662492348084e+09	INFO	Hello World	{"controller": "podset", "controllerGroup": "app.example.com", "controllerKind": "PodSet", "podSet": {"name":"podset-sample","namespace":"default"}, "namespace": "default", "name": "podset-sample", "reconcileID": "5ede544e-8244-461c-b738-9f803d60cc9b"}
```

Our first Operator has reconciled its first CR!

### Cleanup

You can stop a locally running Operator with `Ctrl+C`

If you remove the CR, all of the objects created by its reconcile (none
right now) are deleted.

```
kubectl delete -f config/samples/app_v1alpha1_podset.yaml
```

Finally, remove the CRD.

```
make uninstall
```

## Adding a field to the Spec

In Kubernetes, every functional object (with some exceptions, i.e.
ConfigMap) includes `spec` and `status`. Kubernetes functions by reconciling
desired state (Spec) with the actual cluster state. We then record what
is observed (Status).

Go-based Operators are able to generate and regenerate some of the
crucial files, so you don't have to! Let’s inspect one of the files we
**are** supposed to change, `api/v1alpha1/podset_types.go` which defines
the PodSet API for the auto-generation.

`PodSetSpec` represents the desired state, (input comes from PodSet CR).
Users will need to tell the Operator how many Pods we want, so lets add
`Replicas`.

```
type PodSetSpec struct {
	// Replicas is the desired number of pods for the PodSet
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	Replicas int32 `json:"replicas,omitempty"`
}
```

Notice the `+kubebuilder` comment markers found throughout the file.
Operator-SDK makes use of a tool called `controler-gen` (from the
[controller-tools](https://github.com/kubernetes-sigs/controller-tools)
project) for generating utility code and Kubernetes YAML. More
information on markers for config/code generation can be found
[here](https://book.kubebuilder.io/reference/markers.html).

**Important: Every time you modify a `*_types.go` file, you will need to update the
generated files!**

Regenerate `zz_generated.deepcopy.go` with:

`make generate`

Regenerate object YAMLs (including the CRDs!):

`make manifests`

Thanks to our comment markers, observe that we now have a newly
generated CRD YAML that reflects the `spec.replicas` OpenAPI v3 schema
validation and customized print columns.

`cat config/crd/bases/app.example.com_podsets.yaml`

Next, lets use our the `Replicas` field in our Reconcile loop.
Modify the PodSet controller logic at `controllers/podset_controller.go`:


```
func (r *PodSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Hello World")

	// Fetch the PodSet instance
	instance := &appv1alpha1.PodSet{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found; it could have been deleted after
			// the reconcile request was queued. Owned objects (in our case,
			// Pods) are automatically garbage collected, so there is nothing
			// for us to do. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object. By returning an error, the library will log
		// that error and requeue the resource with backoff logic.
		return ctrl.Result{}, err
	}
	log.Info(fmt.Sprintf("CR has specified %v replicas", instance.Spec.Replicas))
	return ctrl.Result{}, nil
}
```

If you forgot to cleanup after "Hello World", stop the controller with `CTRL+C`, delete the
PodSet CR with `kubectl delete podset podset-sample`, and uninstall the
PodSet CRD with `make uninstall`.

Reinstall the updated CRD, start the controller with `make run`, and in
another session, the CR with `kubectl apply -f config/samples/app_v1alpha1_podset.yaml`

## Reporting back to the user with Status

Our controller is now able to read values from the CR, now it is time to
report back using the `PodSet.Status`. In our case, we want to know how
many Pods are available, and what the Pod names are:

```
// PodSetStatus defines the observed state of PodSet
type PodSetStatus struct {
	PodNames          []string `json:"podNames"`
	AvailableReplicas int32    `json:"availableReplicas"`
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
