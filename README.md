# resource-exporter

Resource Exporter is a Daemonset to collect the device resource information on each node and update it to [CRD](https://github.com/volcano-sh/apis/tree/master/pkg/apis/nodeinfo/v1alpha1) for Volcano scheduling, e.g. NUMA-Aware scheduling.

Notes:

Resource Exporter supports the CPU NUMA topology resource so far.  More resources will be included in the future.

## Compatibility

Resource Exporter does not currently impose a strict Kubernetes version restriction. Kubernetes v1.35.3 in `go.mod` is the dependency version used to build Resource Exporter v1.0.0, not a minimum or exact Kubernetes cluster version.

The kubelet PodResources API was available before Kubernetes v1.28 and graduated to GA in v1.28. Resource Exporter only uses its `List` operation for CPU allocation collection. This collection is optional and can be disabled with `--enable-pod-resource=false`; if the PodResources client cannot be initialized, Resource Exporter falls back to reading `cpu_manager_state`. Users running older Kubernetes versions should verify that the kubelet PodResources API is available or use this compatibility mode.

Resource Exporter v1.0.0 reports the `Numatopology.spec.podAllocations` field, so Volcano v1.15.x and its `Numatopology` CRD are recommended. Future release notes should identify the aligned Kubernetes dependency version, the recommended Volcano and CRD version, and any kubelet API requirements.

## Quick Start Guide

### Compilation
```
   make image [TAG=XXX]
```

### Prerequisites

- Volcano has been installed,  refer to [ volcano Install Guide](https://github.com/volcano-sh/volcano/blob/master/installer/README.md)


### Installation

#### 1. Edit the file [./installer/numa-topo.yaml](https://github.com/volcano-sh/resource-exporter/blob/master/installer/numa-topo.yaml)

There are some options which you can use to configure

|Parameter|Description|Default Value|
|----------------|-----------------|----------------------|
|kubelet-conf|specify kubelet configuration file path to get its configuration|/var/lib/kubelet/config.yaml|
|cpu-manager-state| specify the cpu manager state file path in kubelet to get get the real-time CPU topology data| /var/lib/kubelet/cpu_manager_state|
|device-path|specify the system device path to get the NUMA data of worker node| /sys/devices/system|
|res-reserved| specify the reserved resource of worker node; if the reserved resource is configured in the kubelet configuration file, you can ignore it|""|

#### 2. Deploy resource exporter

````
   kubectl apply -f ./installer/numa-topo.yaml
````

