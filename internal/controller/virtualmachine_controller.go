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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	azurergv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	greenopsv1 "github.com/vicentinileonardo/cloud-resource-manager/api/v1"
)

// VirtualMachineReconciler reconciles a VirtualMachine object
type VirtualMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type Scheduling struct {
	SchedulingRegion string
	SchedulingTime   string
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
	} else {
		l.Info("Provider information available")
	}

	// Log image, cpu, memory, provider, region, time
	l.Info("Image: " + vm.Spec.Image)
	l.Info("Cpu: " + string(vm.Spec.Cpu)) //TODO: fix this
	l.Info("Memory: " + vm.Spec.Memory)
	l.Info("Provider: " + vm.Spec.Provider)

	schedulingInfo := Scheduling{}
	for _, scheduling := range vm.Spec.Scheduling {
		//l.Info(fmt.Sprintf("Type: %s", scheduling.Type))
		//l.Info(fmt.Sprintf("Decision: %s", scheduling.Decision))

		switch scheduling.Type {
		case "region":
			schedulingInfo.SchedulingRegion = scheduling.Decision
		case "time":
			schedulingInfo.SchedulingTime = scheduling.Decision
		default:
			l.Info("Unknown scheduling type")
		}
	}

	l.Info(fmt.Sprintf("SchedulingRegion: %s", schedulingInfo.SchedulingRegion))
	l.Info(fmt.Sprintf("SchedulingTime: %s", schedulingInfo.SchedulingTime))

	// Azure ResourceGroup

	// Check if ResourceGroup already exists
	rg := &azurergv1.ResourceGroup{}
	err = r.Get(ctx, client.ObjectKey{
		Namespace: vm.Namespace,
		Name:      vm.Name + "-rg",
	}, rg)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			l.Info("Azure ResourceGroup not found")
			l.Info("Creating ResourceGroup")
			rg := &azurergv1.ResourceGroup{
				ObjectMeta: ctrl.ObjectMeta{
					Name:      vm.Name + "-rg",
					Namespace: vm.Namespace,
				},
				Spec: azurergv1.ResourceGroup_Spec{
					Location: &schedulingInfo.SchedulingRegion,
				},
			}

			if err := ctrl.SetControllerReference(vm, rg, r.Scheme); err != nil {
				l.Error(err, "Failed to set controller reference for Azure ResourceGroup")
				return ctrl.Result{}, err
			} else {
				l.Info("Set controller reference for Azure ResourceGroup")
			}

			if err := r.Create(ctx, rg); err != nil {
				l.Error(err, "Failed to create Azure ResourceGroup")
				return ctrl.Result{}, err
			}

			l.Info("Created Azure ResourceGroup")

		} else {
			l.Error(err, "Failed to get Azure ResourceGroup")
			return ctrl.Result{}, err
		}
	} else {
		l.Info("Azure ResourceGroup found in the cluster")
	}

	// check if provider specific vm already exists, if not create a new one
	// create the cloud-specific VirtualMachine resource
	// Create or update using CreateOrUpdate
	//op, err := controllerutil.CreateOrUpdate(ctx, r.Client, cloudCR, func() error {
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

/*
func (r *VirtualMachineReconciler) createAndPersistAzureResourceGroup(ctx context.Context, vm *greenopsv1.VirtualMachine, schedulingInfo Scheduling) (*azurergv1.ResourceGroup, error) {
    // ResourceGroup already exists, return the existing one
    //return existingRg, nil
}
*/

/*
func (r *VirtualMachineReconciler) createAzureVirtualMachine(vm *greenopsv1.VirtualMachine, rg *azurergv1.ResourceGroup, schedulingInfo Scheduling) *azurevmv1.VirtualMachine {
	// Set the ResourceGroup as owner using KnownResourceReference
	owner := genruntime.KnownResourceReference{
		Name: rg.Name,
	}

	// Fixed values for now
	vmSize := "Standard_A1_v2"
	publisher := "Canonical"
	offer := "0001-com-ubuntu-server-jammy"
	sku := "22-04-lts"
	version := "latest"
	location := schedulingInfo.SchedulingRegion

	// Create the Azure VM
	azureVM := &azurevmv1.VirtualMachine{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      vm.Name,
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"app":           vm.Name,
				"resourcegroup": rg.Name,
			},
		},
		Spec: azurevmv1.VirtualMachine_Spec{
			Location: &location,
			Owner:    &owner,
			HardwareProfile: &azurevmv1.HardwareProfile{
				VmSize: &vmSize,
			},
			StorageProfile: &azurevmv1.StorageProfile{
				ImageReference: &azurevmv1.ImageReference{
					Publisher: &publisher,
					Offer:     &offer,
					Sku:       &sku,
					Version:   &version,
				},
			},
		},
	}

	// Set the VirtualMachine as the owner of the Azure VM
	if err := ctrl.SetControllerReference(vm, azureVM, r.Scheme); err != nil {
		// In production code, you should handle this error appropriately
		return nil
	}

	return azureVM
}
*/

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Register the Azure Resource Group to the scheme
	if err := azurergv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&greenopsv1.VirtualMachine{}).
		Named("virtualmachine").
		Complete(r)
}
