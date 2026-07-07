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

package numatopo

import (
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	podresv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	"k8s.io/kubernetes/pkg/kubelet/apis/podresources"
)

const (
	defaultConnectionTimeout = 2 * time.Second
	defaultMaxSize           = 1024 * 1024 * 16
)

var (
	client podresv1.PodResourcesListerClient
	conn   *grpc.ClientConn
)

func InitPodResourcesClient(sockDir string) error {
	sockPath := filepath.Join(sockDir, "kubelet.sock")
	var err error
	client, conn, err = podresources.GetV1Client("unix://"+sockPath, defaultConnectionTimeout, defaultMaxSize)
	return err
}

func ClosePodResourcesClient() {
	if conn != nil {
		conn.Close()
	}
}
