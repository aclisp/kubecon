package kubeclient

import (
	"encoding/json"
	"github.com/golang/glog"
	"io/ioutil"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	kube_clientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kube_clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

const (
	APIVersion = "v1"
)

var (
	kubeClient     *kube_client.Client
	KubeConfig     *Config
	KubeConfigFile = "kubeconfig.json"
)

type Config struct {
	APIServerURL string
	Username     string
	Password     string
}

func loadKubeConfig() {
	KubeConfig = &Config{
		APIServerURL: "https://61.160.36.122",
		Username:     "test",
		Password:     "test123",
	}
	cfg, err := ioutil.ReadFile(KubeConfigFile)
	if err != nil {
		glog.Warningf("Can not read %q: %v", KubeConfigFile, err)
		return
	}
	if err := json.Unmarshal(cfg, KubeConfig); err != nil {
		glog.Warningf("Can not unmarshal content of %q: %v", KubeConfigFile, err)
		return
	}
	glog.Infof("Loaded %q", KubeConfigFile)
}

func SaveKubeConfig() {
	data, err := json.MarshalIndent(KubeConfig, "", "  ")
	if err != nil {
		glog.Errorf("Can not marshal kubeconfig: %v", err)
		return
	}
	if err := ioutil.WriteFile(KubeConfigFile, data, 0640); err != nil {
		glog.Errorf("Can not write %q: %v", KubeConfigFile, err)
		return
	}
	glog.Infof("Saved to %q", KubeConfigFile)
}

func Init() {
	var err error
	if KubeConfig == nil {
		loadKubeConfig()
	}
	kubeClient, err = getKubeClient()
	if err != nil {
		glog.Fatalf("Can not connect to kubernetes: %v", err)
	}
}

func Get() *kube_client.Client {
	if kubeClient == nil {
		glog.Fatalf("Forget to call kubeclient.Init()?")
	}
	return kubeClient
}

func getConfigOverrides() (*kube_clientcmd.ConfigOverrides, error) {
	kubeConfigOverride := kube_clientcmd.ConfigOverrides{
		ClusterInfo: kube_clientcmdapi.Cluster{
			APIVersion: APIVersion,
		},
	}

	kubeConfigOverride.ClusterInfo.Server = KubeConfig.APIServerURL
	kubeConfigOverride.ClusterInfo.InsecureSkipTLSVerify = true

	return &kubeConfigOverride, nil
}

func getKubeClient() (*kube_client.Client, error) {
	configOverrides, err := getConfigOverrides()
	if err != nil {
		return nil, err
	}

	kubeConfig := &kube_client.Config{
		Host:     configOverrides.ClusterInfo.Server,
		Version:  configOverrides.ClusterInfo.APIVersion,
		Insecure: configOverrides.ClusterInfo.InsecureSkipTLSVerify,
		Username: KubeConfig.Username,
		Password: KubeConfig.Password,
	}
	kubeClient := kube_client.NewOrDie(kubeConfig)
	return kubeClient, nil
}

func GetAllPods() ([]*api.Pod, error) {
	podList, err := kubeClient.Pods("").List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	var result []*api.Pod
	for j := range podList.Items {
		pod := &podList.Items[j]
		result = append(result, pod)
	}
	return result, nil
}
