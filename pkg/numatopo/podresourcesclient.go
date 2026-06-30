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
