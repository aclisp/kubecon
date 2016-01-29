package main

import (
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	api_uv "k8s.io/kubernetes/pkg/api/unversioned"
	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	kube_clientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kube_clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

const (
	APIVersion        = "v1"
	PrivateRepoPrefix = "61.160.36.122:8080/"
)

var (
	kubeClient  *kube_client.Client
	portMapping *regexp.Regexp
)

func main() {
	defer glog.Flush()

	flag.Set("logtostderr", "true")
	flag.Parse()

	var err error
	kubeClient, err = getKubeClient()
	if err != nil {
		glog.Fatalf("Can not connect to kubernetes: %v", err)
	}
	portMapping = regexp.MustCompile(`PortMapping\((.*)\)`)

	r := gin.Default()
	r.Static("/js", "js")
	r.Static("/css", "css")
	r.Static("/fonts", "fonts")
	r.LoadHTMLGlob("pages/*")

	r.GET("/namespaces/:ns/pods", listPodsInNamespace)

	r.Run(":8080") // listen and serve on 0.0.0.0:8080
}

func listPodsInNamespace(c *gin.Context) {
	ns := c.Param("ns")
	list, err := kubeClient.Pods(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "podList", gin.H{
		"title": "Show Pods",
		"pods":  genPods(list),
	})
}

func getConfigOverrides() (*kube_clientcmd.ConfigOverrides, error) {
	kubeConfigOverride := kube_clientcmd.ConfigOverrides{
		ClusterInfo: kube_clientcmdapi.Cluster{
			APIVersion: APIVersion,
		},
	}

	kubeConfigOverride.ClusterInfo.Server = fmt.Sprintf("%s://%s", "https", "61.160.36.122")
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
		Username: "test",
		Password: "test123",
	}
	kubeClient := kube_client.NewOrDie(kubeConfig)
	return kubeClient, nil
}

type Pod struct {
	Name            string
	Image           string
	PrivateRepo     bool
	TotalContainers int
	ReadyContainers int
	Status          string
	Restarts        int
	Age             string
	HostNetwork     bool
	HostIP          string
	PodIP           string
	Ports           string
}

func genPods(list *api.PodList) (pods []Pod) {
	for i := range list.Items {
		pods = append(pods, genOnePod(&list.Items[i]))
	}
	return
}

func genOnePod(pod *api.Pod) Pod {
	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]

		restarts += container.RestartCount
		if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
			reason = container.State.Waiting.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
			reason = container.State.Terminated.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
			if container.State.Terminated.Signal != 0 {
				reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
			} else {
				reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
			}
		} else if container.Ready && container.State.Running != nil {
			readyContainers++
		}
	}
	if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}
	podIP := ""
	ports := ""
	if pod.Spec.HostNetwork {
		podIP = ""
		for i := range pod.Spec.Containers {
			for j := range pod.Spec.Containers[i].Ports {
				port := pod.Spec.Containers[i].Ports[j]
				ports += fmt.Sprintf("%d/%s,", port.HostPort, port.Protocol)
			}
		}
		ports = strings.TrimSuffix(ports, ",")
	} else {
		podIP = pod.Status.PodIP
		ports = portMapping.FindStringSubmatch(pod.Status.Message)[1]
	}
	image := pod.Spec.Containers[0].Image
	privateRepo := false
	if strings.HasPrefix(image, PrivateRepoPrefix) {
		image = strings.TrimPrefix(image, PrivateRepoPrefix)
		privateRepo = true
	}

	return Pod{
		Name:            pod.Name,
		Image:           image,
		PrivateRepo:     privateRepo,
		TotalContainers: totalContainers,
		ReadyContainers: readyContainers,
		Status:          reason,
		Restarts:        restarts,
		Age:             translateTimestamp(pod.CreationTimestamp),
		HostNetwork:     pod.Spec.HostNetwork,
		HostIP:          pod.Spec.NodeName,
		PodIP:           podIP,
		Ports:           ports,
	}
}

// translateTimestamp returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestamp(timestamp api_uv.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return shortHumanDuration(time.Now().Sub(timestamp.Time))
}

func shortHumanDuration(d time.Duration) string {
	// Allow deviation no more than 2 seconds(excluded) to tolerate machine time
	// inconsistence, it can be considered as almost now.
	if seconds := int(d.Seconds()); seconds < -1 {
		return fmt.Sprintf("<invalid>")
	} else if seconds < 0 {
		return fmt.Sprintf("0s")
	} else if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if minutes := int(d.Minutes()); minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	} else if hours := int(d.Hours()); hours < 24 {
		return fmt.Sprintf("%dh", hours)
	} else if hours < 24*364 {
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}
