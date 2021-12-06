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

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "dev.quack.dev/guestbook/api/v1"
	istionetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istiov1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FMAppReconciler reconciles a FMApp object
type FMAppReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.dev.quack.dev,resources=fmapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.dev.quack.dev,resources=fmapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.dev.quack.dev,resources=fmapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=,resources=service,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FMApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *FMAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	log := log.Log.WithName("controllers").WithName("FMApp")
	// set a label for our deployment
	labels := map[string]string{
		"app":        "fmapp",
		"controller": req.Name,
	}

	var fmapp appsv1.FMApp
	log.Info("fetching Foo Resource")
	if err := r.Get(ctx, req.NamespacedName, &fmapp); err != nil {
		log.Error(err, "unable to fetch Foo")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	vs := CreateVirtualService(&fmapp)
	/*
		### 2: Clean Up old Deployment which had been owned by FMApp Resource.
		We'll find deployment object which foo object owns.
	*/
	if err := r.cleanupOwnedResources(ctx, log, &fmapp, req); err != nil {
		log.Error(err, "failed to clean up old Deployment resources for this FMApp")
		return ctrl.Result{}, err
	}
	/*
		### 3: Create or Update deployment object which match foo.Spec.
		We'll use ctrl.CreateOrUpdate method.
		It enable us to create an object if it doesn't exist.
		Or it enable us to update the object if it exists.
	*/

	// get deploymentName from foo.Spec
	deploymentName := fmapp.Spec.DeploymentName

	// define deployment template using deploymentName
	deploy := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: req.Namespace,
		},
	}
	// define service template using deploymentName
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: req.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}

	// Create or Update deployment object
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		// set the replicas from foo.Spec
		replicas := int32(1)
		if fmapp.Spec.Replicas != nil {
			replicas = *fmapp.Spec.Replicas
		}
		deploy.Spec.Replicas = &replicas

		// set labels to spec.selector for our deployment
		if deploy.Spec.Selector == nil {
			deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		}

		// set labels to template.objectMeta for our deployment
		if deploy.Spec.Template.ObjectMeta.Labels == nil {
			deploy.Spec.Template.ObjectMeta.Labels = labels
		}

		// set a container for our deployment
		containers := []corev1.Container{
			{
				Name:  "httpbin",
				Image: "docker.io/kennethreitz/httpbin",
			},
		}

		// set containers to template.spec.containers for our deployment
		if deploy.Spec.Template.Spec.Containers == nil {
			deploy.Spec.Template.Spec.Containers = containers
		}

		// set the owner so that garbage collection can kicks in
		if err := ctrl.SetControllerReference(&fmapp, deploy, r.Scheme); err != nil {
			log.Error(err, "unable to set ownerReference from Foo to Deployment")
			return err
		}

		// end of ctrl.CreateOrUpdate
		return nil
	}); err != nil {

		log.Error(err, "unable to ensure deployment is correct")
		r.Recorder.Eventf(&fmapp, corev1.EventTypeWarning, "Deployment", "unable to ensure deployment is correct %s", err.Error())
		return ctrl.Result{}, err

	}
	// Create or Update service  object
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
		// set labels to spec.selector for our deployment
		if service.Spec.Selector == nil {
			service.Spec.Selector = labels
		}

		// set the owner so that garbage collection can kicks in
		if err := ctrl.SetControllerReference(&fmapp, service, r.Scheme); err != nil {
			log.Error(err, "unable to set ownerReference from fmapp to Deployment")
			return err
		}

		// end of ctrl.CreateOrUpdate
		return nil
	}); err != nil {

		// error handling of ctrl.CreateOrUpdate
		log.Error(err, "unable to ensure service is correct")
		r.Recorder.Eventf(&fmapp, corev1.EventTypeWarning, "Service", "unable to ensure service is correct %s", err.Error())
		return ctrl.Result{}, err

	}

	err := r.Get(ctx, req.NamespacedName, vs)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a VS")
		err = r.Create(ctx, vs)
		if err != nil {
			log.Error(err, "unable to create virtual reference")
			r.Recorder.Eventf(&fmapp, corev1.EventTypeWarning, "VirtualService", "Unable to create VS: %s", err.Error())
			return ctrl.Result{}, err
		}
		// set the owner so that garbage collection can kicks in
		if err := ctrl.SetControllerReference(&fmapp, vs, r.Scheme); err != nil {
			log.Error(err, "unable to set ownerReference from fmapp to Deployment")
			return ctrl.Result{}, err
		}
	}

	/*
		### 4: Update foo status.
		First, we get deployment object from in-memory-cache.
		Second, we get deployment.status.AvailableReplicas in order to update foo.status.AvailableReplicas.
		Third, we update foo.status from deployment.status.AvailableReplicas.
		Finally, finish reconcile. and the next reconcile loop would start unless controller process ends.
	*/

	// get deployment object from in-memory-cache
	var deployment apps.Deployment
	deploymentNamespacedName := client.ObjectKey{Namespace: req.Namespace, Name: fmapp.Spec.DeploymentName}
	if err := r.Get(ctx, deploymentNamespacedName, &deployment); err != nil {
		log.Error(err, "unable to fetch Deployment")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// set foo.status.AvailableReplicas from deployment
	availableReplicas := deployment.Status.AvailableReplicas
	if availableReplicas == fmapp.Status.AvailableReplicas {
		return ctrl.Result{}, nil
	}
	fmapp.Status.AvailableReplicas = availableReplicas

	// update foo.status
	if err := r.Status().Update(ctx, &fmapp); err != nil {
		log.Error(err, "unable to update Foo status")
		return ctrl.Result{}, err
	}

	// create event for updated foo.status
	r.Recorder.Eventf(&fmapp, corev1.EventTypeNormal, "Updated", "Update fm.status.AvailableReplicas: %d", fmapp.Status.AvailableReplicas)

	return ctrl.Result{}, nil
}

func (r *FMAppReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, foo *appsv1.FMApp, req ctrl.Request) error {
	log.Info("finding existing Deployments for Foo resource")

	// List all deployment resources owned by this Foo
	var deployments apps.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(foo.Namespace),
		client.MatchingFields(map[string]string{deploymentOwnerKey: foo.Name})); err != nil {
		return nil
	}

	// Delete deployment if the deployment name doesn't match foo.spec.deploymentName
	for _, deployment := range deployments.Items {
		if deployment.Name == foo.Spec.DeploymentName {
			// If this deployment's name matches the one on the Foo resource
			// then do not delete it.
			continue
		}

		// Delete old deployment object which doesn't match foo.spec.deploymentName
		if err := r.Delete(ctx, &deployment); err != nil {
			log.Error(err, "failed to delete Deployment resource")
			return err
		}

		log.Info("delete deployment resource: " + deployment.Name)
	}

	// List all deployment resources owned by this Foo
	var service v1.ServiceList
	if err := r.List(ctx, &service, client.InNamespace(foo.Namespace), client.MatchingFields(map[string]string{deploymentOwnerKey: foo.Name})); err != nil {
		return nil
	}

	// Delete service  if the service  name doesn't match foo.spec.deploymentName
	for _, s := range service.Items {
		if s.Name == foo.Spec.DeploymentName {
			// If this service's name matches the one on the Foo resource
			// then do not delete it.
			continue
		}

		// Delete old deployment object which doesn't match foo.spec.deploymentName
		if err := r.Delete(ctx, &s); err != nil {
			log.Error(err, "failed to delete service resource")
			return err
		}
	}
	vs := CreateVirtualService(foo)
	err := r.Get(context.Background(), req.NamespacedName, vs)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return r.Delete(ctx, vs)
}

var (
	deploymentOwnerKey = ".metadata.controller"
	apiGVStr           = appsv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *FMAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.FMApp{}).
		Complete(r)
}

func CreateVirtualService(workload *appsv1.FMApp) *istiov1alpha3.VirtualService {
	hostname := "httpbin.example.com"
	vs := &istiov1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.GetName(),
			Namespace: workload.Namespace,
		},
		Spec: istionetworkingv1alpha3.VirtualService{
			Hosts:    []string{hostname},
			Gateways: []string{"httpbin-gateway"},
			Http: []*istionetworkingv1alpha3.HTTPRoute{{
				Match: []*istionetworkingv1alpha3.HTTPMatchRequest{{
					Uri: &istionetworkingv1alpha3.StringMatch{
						MatchType: &istionetworkingv1alpha3.StringMatch_Prefix{
							Prefix: "/status",
						},
					},
				}},
				Route: []*istionetworkingv1alpha3.HTTPRouteDestination{{
					Destination: &istionetworkingv1alpha3.Destination{
						Port: &istionetworkingv1alpha3.PortSelector{
							Number: uint32(80),
						},
						Host: "httpbin",
					},
				}},
			}},
		},
	}

	return vs
}
