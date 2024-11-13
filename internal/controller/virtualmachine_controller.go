/*
Copyright 2024.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	greenopsv1 "github.com/vicentinileonardo/cloud-resource-manager/api/v1"
)

// VirtualMachineReconciler reconciles a VirtualMachine object
type VirtualMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=greenops.greenops.test,resources=virtualmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=greenops.greenops.test,resources=virtualmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=greenops.greenops.test,resources=virtualmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VirtualMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *VirtualMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// TODO(user): your logic here

	l.Info("Reconciling VirtualMachine")

	// Fetch the VirtualMachine instance
	vm := &greenopsv1.VirtualMachine{}
	err := r.Get(ctx, req.NamespacedName, vm)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			l.Info("VirtualMachine resource not found")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get VirtualMachine")
		return ctrl.Result{}, err
	}

	// Check if scheduling information is available
	if vm.Spec.Provider == "" {
		l.Info("Provider information not yet available, requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil //interval to be defined
	}

	// Log image, cpu, memory, provider, region, time
	if vm.Spec.Image != "" {
		l.Info("Image: " + vm.Spec.Image)
	}
	if vm.Spec.Cpu != 0 {
		l.Info("Cpu: " + string(vm.Spec.Cpu)) //TODO: fix this
	}
	if vm.Spec.Memory != "" {
		l.Info("Memory: " + vm.Spec.Memory)
	}
	if vm.Spec.Provider != "" {
		l.Info("Provider: " + vm.Spec.Provider)
	}
	for _, scheduling := range vm.Spec.Scheduling {
		l.Info("Type: " + scheduling.Type)
		l.Info("Decision: " + scheduling.Decision)
	}

	// check if provider specific resource already exists, if not create a new one

	// create the cloud-specific VirtualMachine resource

	//at the end set status field provisioned to true
	vm.Status.Provisioned = true
	err = r.Status().Update(ctx, vm)
	if err != nil {
		l.Error(err, "Failed to update VirtualMachine status")
		return ctrl.Result{}, err
	} else {
		l.Info("VirtualMachine status updated")
	}

	l.Info("End Reconciling VirtualMachine")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&greenopsv1.VirtualMachine{}).
		Named("virtualmachine").
		Complete(r)
}
