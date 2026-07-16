/*
Copyright 2021 The Volcano Authors.

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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"

	"volcano.sh/apis/pkg/client/clientset/versioned"

	"volcano.sh/resource-exporter/pkg/args"
	"volcano.sh/resource-exporter/pkg/machineinfo"
	"volcano.sh/resource-exporter/pkg/numatopo"
)

var logFlushFreq = pflag.Duration("log-flush-frequency", 5*time.Second, "Maximum number of seconds between log flushes")

func getNumaTopoClient(argument *args.Argument) (*versioned.Clientset, error) {
	config, err := args.BuildConfig(argument.KubeClientOptions)
	if err != nil {
		return nil, err
	}

	return versioned.NewForConfigOrDie(config), err
}

func main() {
	klog.InitFlags(nil)

	opt := args.NewArgument()
	opt.AddFlags(pflag.CommandLine)
	cliflag.InitFlags()

	go wait.Until(klog.Flush, *logFlushFreq, wait.NeverStop)
	defer klog.Flush()

	// load machine info, if this fails, will go into panic.
	err := machineinfo.InitializeMachineInfo()
	if err != nil {
		klog.Fatal(err)
	}

	// Get hostname from environment variable
	hostname := os.Getenv("MY_NODE_NAME")
	if hostname == "" {
		klog.Fatal("MY_NODE_NAME environment variable is required")
	}

	nodeInfoClient, err := getNumaTopoClient(opt)
	if err != nil {
		klog.Errorf("Get numainfo client failed, err = %v", err)
		return
	}

	if opt.EnableGetCpuIDByPodResourceList {
		err = numatopo.InitPodResourcesClient(opt.PodResourceSockPath)
		if err != nil {
			// Fall back to the cpu_manager_state file-based method instead of
			// leaving the client nil: a nil client would make every NUMA
			// allocation update fail, so the node topology would silently stop
			// being reported. Degrading keeps the daemon reporting, just via the
			// legacy (less reliable) source.
			klog.Errorf("Failed to init podresources client, falling back to cpu_manager_state method: %v", err)
			opt.EnableGetCpuIDByPodResourceList = false
		} else {
			defer numatopo.ClosePodResourcesClient()
		}
	}

	// Initialize Numatopology informer cache
	// This uses list-watch mechanism instead of polling
	numaCache := numatopo.NewNumatopoCache(nodeInfoClient, hostname)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	stopCh := ctx.Done()

	// Start the informer (non-blocking, runs in background goroutine)
	numaCache.Start(stopCh)

	// Wait for informer cache to sync (blocking until initial list is complete)
	if !numaCache.WaitForCacheSync(stopCh) {
		klog.Fatal("Failed to sync Numatopology informer cache")
	}
	klog.V(2).Infof("Numatopology informer cache synced successfully")

	// Use wait.UntilWithContext to periodically check and update Numatopology
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		// Get current resource from informer cache
		cached, err := numaCache.Get()
		exist := true
		if err != nil {
			if apierrors.IsNotFound(err) {
				exist = false
			} else {
				klog.Errorf("Get Numatopology from cache failed, err=%v", err)
				return
			}
		}

		// Check local resource changes, including kubelet config, node numa topologies and pod resource allocations
		isChg := numatopo.NodeInfoRefresh(opt)
		klog.V(4).Infof("Local resource changes within the interval: %v", isChg)

		// Create or update if there are changes or resources non-existent
		if isChg || !exist {
			klog.V(4).Infof("Node info changes detected, updating Numatopology.")
			numatopo.CreateOrUpdateNumatopo(nodeInfoClient, cached)
		}
	}, opt.CheckInterval)
}
