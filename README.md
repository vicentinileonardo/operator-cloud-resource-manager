# operator-cloud-resource-manager

This repo was created in the context of my master's thesis. Currently in standby since another solution was chosen.


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


```bash
go get github.com/Azure/azure-service-operator/v2/api/compute/v1api20220301
go mod tidy
```