# resource-exporter

Resource Exporter is a Daemonset to collect the device resource information on each node and update it to [CRD](https://github.com/volcano-sh/apis/tree/master/pkg/apis/nodeinfo/v1alpha1) for Volcano scheduling, e.g. NUMA-Aware scheduling.

Notes:

Resource Exporter supports the CPU NUMA topology resource so far.  More resources will be included in the future.

## Quick Start Guide

### Compilation
```
   make image [TAG=XXX]
```

### Prerequisites

- Volcano has been installed,  refer to [ volcano Install Guide](https://github.com/volcano-sh/volcano/blob/master/installer/README.md)


### Installation

#### 1. Edit the file ./installer/numa-topo.yaml

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

