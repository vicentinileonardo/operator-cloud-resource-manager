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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	azurevmv1 "github.com/Azure/azure-service-operator/v2/api/compute/v1api20220301"
	azurergv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	greenopsv1 "github.com/vicentinileonardo/cloud-resource-manager/api/v1"
)

const (
	requeueInterval = 30 * time.Second
	vmSuffix        = "-vm"
	rgSuffix        = "-rg"
)

// VirtualMachineReconciler reconciles a VirtualMachine object
type VirtualMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// VMSchedulingInfo contains the scheduling decisions for the VM
type VMSchedulingInfo struct {
	Region string
	Time   string
}

// AzureVMConfig contains the configuration for Azure VM creation
type AzureVMConfig struct {
	VMSize    string
	Publisher string
	Offer     string
	SKU       string
	Version   string
	Location  string
}

// DefaultAzureVMConfig returns the default Azure VM configuration
func DefaultAzureVMConfig(location string) AzureVMConfig {
	return AzureVMConfig{
		VMSize:    "Standard_A1_v2",
		Publisher: "Canonical",
		Offer:     "0001-com-ubuntu-server-jammy",
		SKU:       "22-04-lts",
		Version:   "latest",
		Location:  location,
	}
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
	l.Info("Starting VirtualMachine reconciliation")

	// Fetch the VirtualMachine instance
	vm := &greenopsv1.VirtualMachine{}
	if err := r.Get(ctx, req.NamespacedName, vm); err != nil {
		return r.handleGetError(err, l)
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

	schedulingInfo, err := r.extractSchedulingInfo(vm)
	if err != nil {
		l.Info("Scheduling information not complete")
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// Create or update Azure resources
	if err := r.reconcileAzureResources(ctx, vm, schedulingInfo, l); err != nil {
		l.Error(err, "Failed to reconcile Azure resources")
		return ctrl.Result{}, err
	}

	// Update VM status
	if err := r.updateVMStatus(ctx, vm); err != nil {
		l.Error(err, "Failed to update VM status")
		return ctrl.Result{}, err
	}

	l.Info("End Reconciling VirtualMachine")

	return ctrl.Result{}, nil
}

// handleGetError handles errors when getting the VirtualMachine resource
func (r *VirtualMachineReconciler) handleGetError(err error, l logr.Logger) (ctrl.Result, error) {
	if client.IgnoreNotFound(err) == nil {
		l.Info("VirtualMachine resource not found")
		return ctrl.Result{}, nil
	}
	l.Error(err, "Failed to get VirtualMachine")
	return ctrl.Result{}, err
}

// extractSchedulingInfo extracts scheduling information from the VM spec
func (r *VirtualMachineReconciler) extractSchedulingInfo(vm *greenopsv1.VirtualMachine) (*VMSchedulingInfo, error) {
	info := &VMSchedulingInfo{}

	for _, scheduling := range vm.Spec.Scheduling {
		switch scheduling.Type {
		case "region":
			info.Region = scheduling.Decision
		case "time":
			info.Time = scheduling.Decision
		}
	}

	if info.Region == "" || info.Time == "" {
		return nil, fmt.Errorf("incomplete scheduling information")
	}

	return info, nil
}

// reconcileAzureResources handles the creation and management of Azure-specific resources
func (r *VirtualMachineReconciler) reconcileAzureResources(ctx context.Context, vm *greenopsv1.VirtualMachine, schedulingInfo *VMSchedulingInfo, l logr.Logger) error {
	// Create or update Resource Group
	rg, err := r.reconcileResourceGroup(ctx, vm, schedulingInfo, l)
	if err != nil {
		return fmt.Errorf("failed to reconcile resource group: %w", err)
	}

	// Create or update Virtual Machine
	if err := r.reconcileAzureVM(ctx, vm, rg, schedulingInfo); err != nil {
		return fmt.Errorf("failed to reconcile virtual machine: %w", err)
	}

	return nil
}

func (r *VirtualMachineReconciler) reconcileResourceGroup(ctx context.Context, vm *greenopsv1.VirtualMachine, schedulingInfo *VMSchedulingInfo, l logr.Logger) (*azurergv1.ResourceGroup, error) {
	rg := &azurergv1.ResourceGroup{}
	rgName := vm.Name + rgSuffix

	err := r.Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: rgName}, rg)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Resource group doesn't exist, create it
			rg = &azurergv1.ResourceGroup{
				ObjectMeta: ctrl.ObjectMeta{
					Name:      rgName,
					Namespace: vm.Namespace,
				},
				Spec: azurergv1.ResourceGroup_Spec{
					Location: &schedulingInfo.Region,
				},
			}

			if err := ctrl.SetControllerReference(vm, rg, r.Scheme); err != nil {
				return nil, fmt.Errorf("failed to set controller reference: %w", err)
			}

			if err := r.Create(ctx, rg); err != nil {
				return nil, fmt.Errorf("failed to create resource group: %w", err)
			}

			return rg, nil
		}
		return nil, fmt.Errorf("failed to get resource group: %w", err)
	}

	// Resource group exists, check if it needs updates
	if rg.Spec.Location == nil || *rg.Spec.Location != schedulingInfo.Region {
		rg.Spec.Location = &schedulingInfo.Region
		if err := r.Update(ctx, rg); err != nil {
			return nil, fmt.Errorf("failed to update resource group: %w", err)
		}
	}

	return rg, nil
}

func (r *VirtualMachineReconciler) reconcileAzureVM(ctx context.Context, vm *greenopsv1.VirtualMachine, rg *azurergv1.ResourceGroup, schedulingInfo *VMSchedulingInfo) error {
	azureVM := &azurevmv1.VirtualMachine{}
	vmName := vm.Name + vmSuffix

	err := r.Get(ctx, client.ObjectKey{Namespace: vm.Namespace, Name: vmName}, azureVM)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// VM doesn't exist, create it
			config := DefaultAzureVMConfig(schedulingInfo.Region)
			azureVM = r.createAzureVMSpec(vm, rg, config)

			if err := ctrl.SetControllerReference(vm, azureVM, r.Scheme); err != nil {
				return fmt.Errorf("failed to set controller reference: %w", err)
			}

			if err := r.Create(ctx, azureVM); err != nil {
				return fmt.Errorf("failed to create Azure VM: %w", err)
			}

			return nil
		}
		return fmt.Errorf("failed to get Azure VM: %w", err)
	}

	// VM exists, check if it needs updates
	config := DefaultAzureVMConfig(schedulingInfo.Region)
	if needsUpdate := r.vmNeedsUpdate(azureVM, config); needsUpdate {
		azureVM = r.updateAzureVMSpec(azureVM, config)
		if err := r.Update(ctx, azureVM); err != nil {
			return fmt.Errorf("failed to update Azure VM: %w", err)
		}
	}

	return nil
}

// Helper function to check if VM needs updates
func (r *VirtualMachineReconciler) vmNeedsUpdate(vm *azurevmv1.VirtualMachine, config AzureVMConfig) bool {
	if vm.Spec.Location == nil || *vm.Spec.Location != config.Location {
		return true
	}
	if vm.Spec.HardwareProfile == nil ||
		vm.Spec.HardwareProfile.VmSize == nil ||
		*vm.Spec.HardwareProfile.VmSize != config.VMSize {
		return true
	}
	// Add more conditions as needed
	return false
}

// Helper function to update VM spec
func (r *VirtualMachineReconciler) updateAzureVMSpec(existingVM *azurevmv1.VirtualMachine, config AzureVMConfig) *azurevmv1.VirtualMachine {
	// Update only the fields that can be modified
	existingVM.Spec.Location = &config.Location
	if existingVM.Spec.HardwareProfile == nil {
		existingVM.Spec.HardwareProfile = &azurevmv1.HardwareProfile{}
	}
	existingVM.Spec.HardwareProfile.VmSize = &config.VMSize

	return existingVM
}

// createAzureVMSpec creates the Azure VM specification
func (r *VirtualMachineReconciler) createAzureVMSpec(vm *greenopsv1.VirtualMachine, rg *azurergv1.ResourceGroup, config AzureVMConfig) *azurevmv1.VirtualMachine {
	return &azurevmv1.VirtualMachine{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      vm.Name + vmSuffix,
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"app":           vm.Name,
				"resourcegroup": rg.Name,
			},
		},
		Spec: azurevmv1.VirtualMachine_Spec{
			Location: &config.Location,
			Owner: &genruntime.KnownResourceReference{
				Name: rg.Name,
			},
			HardwareProfile: &azurevmv1.HardwareProfile{
				VmSize: &config.VMSize,
			},
			StorageProfile: &azurevmv1.StorageProfile{
				ImageReference: &azurevmv1.ImageReference{
					Publisher: &config.Publisher,
					Offer:     &config.Offer,
					Sku:       &config.SKU,
					Version:   &config.Version,
				},
			},
		},
	}
}

// updateVMStatus updates the status of the VirtualMachine resource (Generic VM, not cloud specific)
func (r *VirtualMachineReconciler) updateVMStatus(ctx context.Context, vm *greenopsv1.VirtualMachine) error {
	vm.Status.Provisioned = true
	return r.Status().Update(ctx, vm)
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Register the Azure ResourceGroup to the scheme
	if err := azurergv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	// Register the Azure VirtualMachine to the scheme
	if err := azurevmv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&greenopsv1.VirtualMachine{}).
		Named("virtualmachine").
		Complete(r)
}
