package kubeclient

import (
	"github.com/golang/glog"

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
	kubeClient *kube_client.Client
	KubeConfig = &Config{
		APIServerURL: "https://61.160.36.122",
		Username:     "test",
		Password:     "test123",
	}
)

type Config struct {
	APIServerURL string
	Username     string
	Password     string
}

func Init() {
	var err error
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
	/*
		namespaces, err := kubeClient.Namespaces().List(labels.Everything(), fields.Everything())
		if err != nil {
			return nil, err
		}
		var result []*api.Pod
		for i := range namespaces.Items {
			namespace := namespaces.Items[i].Name
			podList, err := kubeClient.Pods(namespace).List(labels.Everything(), fields.Everything())
			if err != nil {
				glog.Errorf("Can not get pods in namespace '%s': %v", namespace, err)
				continue
			}
			for j := range podList.Items {
				pod := &podList.Items[j]
				result = append(result, pod)
			}
		}
		return result, nil
	*/
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
