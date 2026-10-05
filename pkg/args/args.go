package args

import (
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
)

const (
	defaultCheckInterval = 3 * time.Second
)

// ClientOptions used to build kube rest config.
type ClientOptions struct {
	Master     string
	KubeConfig string
}

// DefaultPCIDevicePath is the sysfs directory scanned for GPU topology when
// --pci-device-path is not set. Inside a container this is the container's own
// sysfs view; set the flag to a host mount when the runtime masks /sys.
const DefaultPCIDevicePath = "/sys/bus/pci/devices"

// Argument is the object to save config set
type Argument struct {
	CheckInterval          time.Duration
	KubeletConf            string
	DevicePath             string
	PCIDevicePath          string
	PodResourceSockPath    string
	CPUMngState            string
	DevicePluginCheckpoint string
	ResReserved            map[string]string
	KubeClientOptions      ClientOptions

	// EnableGetCpuIDByPodResourceList enable get cpu id by PodResourcesLister API
	EnableGetCpuIDByPodResourceList bool
}

// NewArgument init the struct
func NewArgument() *Argument {
	return &Argument{
		ResReserved: make(map[string]string),
	}
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet.
func (args *Argument) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&args.CheckInterval, "check-period", defaultCheckInterval, "Burst to use while talking with kubernetes apiserver")
	fs.StringVar(&args.KubeletConf, "kubelet-conf", args.KubeletConf, "Path to kubelet configure file")
	fs.StringVar(&args.DevicePath, "device-path", args.DevicePath, "Path to device information")
	fs.StringVar(&args.PCIDevicePath, "pci-device-path", DefaultPCIDevicePath, "Path to the sysfs PCI device directory used for GPU topology discovery")
	fs.StringVar(&args.CPUMngState, "cpu-manager-state", args.CPUMngState, "Path to cpu_manager_state")
	fs.StringVar(&args.DevicePluginCheckpoint, "device-plugin-checkpoint", args.DevicePluginCheckpoint, "Path to kubelet_internal_checkpoint")
	fs.Var(cliflag.NewMapStringString(&args.ResReserved), "res-reserved", "kubelet reserved resource  (e.g. cpu=200m,memory=500Mi")

	fs.StringVar(&args.KubeClientOptions.Master, "master", args.KubeClientOptions.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&args.KubeClientOptions.KubeConfig, "kubeconfig", args.KubeClientOptions.KubeConfig, "Path to kubeconfig file with authorization and master location information.")

	fs.StringVar(&args.PodResourceSockPath, "pod-resource-sock", "/var/lib/kubelet/pod-resources", "Path to pod-resource-sock")
	fs.BoolVar(&args.EnableGetCpuIDByPodResourceList, "enable-pod-resource", true, "Enable get cpu id by PodResourcesLister; it is true by default")
}

// BuildConfig builds kube rest config with the given options.
func BuildConfig(opt ClientOptions) (*rest.Config, error) {
	var cfg *rest.Config
	var err error

	master := opt.Master
	kubeconfig := opt.KubeConfig
	cfg, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
