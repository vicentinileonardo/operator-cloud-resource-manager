# operator-cloud-resource-manager

This repo serves as a learning environment for kubebuilder


# Setup (example)
```bash
go mod init github.com/vicentinileonardo/cloud-resource-manager

kubebuilder init --domain greenops.test

kubebuilder create api \ 
	--group greenops
	--kind VirtualMachine
	--version v1 

# after editing the API definitions, generate the manifests such as Custom Resources (CRs) or Custom Resource Definitions (CRDs)
make manifests

make install #to apply crds

# run the controller locally against the remote cluster
make run

# apply sample custom resource
kubectl apply -k config/samples/

kubectl get virtualmachines

kubectl describe virtualmachine virtualmachine-sample
```