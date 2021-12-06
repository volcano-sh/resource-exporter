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
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"

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

func numatopoIsExist(client *versioned.Clientset) (bool, error) {
	hostname := os.Getenv("MY_NODE_NAME")
	if hostname == "" {
		return false, fmt.Errorf("get Hostname failed")
	}

	_, err := client.NodeinfoV1alpha1().Numatopologies().Get(context.TODO(), hostname, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
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

	nodeInfoClient, err := getNumaTopoClient(opt)
	if err != nil {
		klog.Errorf("Get numainfo client failed, err = %v", err)
		return
	}

	tick := time.NewTicker(opt.CheckInterval)
	for {
		select {
		case <-tick.C:
			exist, err := numatopoIsExist(nodeInfoClient)
			if err != nil {
				klog.Errorf("Get numatopo failed, err= %v", err)
				continue
			}

			isChg := numatopo.NodeInfoRefresh(opt)
			if isChg || !exist {
				klog.V(4).Infof("Node info changes.")
				numatopo.CreateOrUpdateNumatopo(nodeInfoClient)
			}
		}
	}
}
