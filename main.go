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
	"k8s.io/kubernetes/pkg/util/sets"
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

	r.GET("/", index)
	r.GET("/namespaces/:ns/pods", listPodsInNamespace)
	r.GET("/nodes", listNodes)
	r.GET("/namespaces/:ns", listOthersInNamespace)

	r.Run(":8080") // listen and serve on 0.0.0.0:8080
}

func index(c *gin.Context) {
	namespaces, err := kubeClient.Namespaces().List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	summary := Summary{}
	for i := range namespaces.Items {
		name := namespaces.Items[i].Name
		podList, err := kubeClient.Pods(name).List(labels.Everything(), fields.Everything())
		if err != nil {
			continue
		}
		summary.Namespaces = append(summary.Namespaces, Namespace{
			Name:     name,
			PodCount: len(podList.Items),
		})
	}
	nodeList, err := kubeClient.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	summary.NodeCount = len(nodeList.Items)

	c.HTML(http.StatusOK, "index", gin.H{
		"title":   "Summary",
		"summary": summary,
	})
}

func listNodes(c *gin.Context) {
	list, err := kubeClient.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "nodeList", gin.H{
		"title": "Nodes",
		"nodes": genNodes(list),
	})
}

func listPodsInNamespace(c *gin.Context) {
	ns := c.Param("ns")
	labelSelectorString, ok := c.GetQuery("labelSelector")
	var labelSelector labels.Selector
	if !ok {
		labelSelector = labels.Everything()
	} else {
		var err error
		if labelSelector, err = labels.Parse(labelSelectorString); err != nil {
			c.HTML(http.StatusBadRequest, "error", gin.H{"error": err.Error()})
			return
		}
	}

	list, err := kubeClient.Pods(ns).List(labelSelector, fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "podList", gin.H{
		"title": "Pods",
		"pods":  genPods(list),
	})
}

func listOthersInNamespace(c *gin.Context) {
	ns := c.Param("ns")
	rcList, err := kubeClient.ReplicationControllers(ns).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	svcList, err := kubeClient.Services(ns).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	epList, err := kubeClient.Endpoints(ns).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "nsInfo", gin.H{
		"title": ns,
		"ns":    ns,
		"rcs":   genReplicationControllers(rcList),
		"svcs":  genServices(svcList),
		"eps":   genEndpoints(epList),
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

type Node struct {
	Name   string
	Status []string
	Age    string
	Labels map[string]string
}

type Namespace struct {
	Name     string
	PodCount int
}

type Summary struct {
	Namespaces []Namespace
	NodeCount  int
}

type ReplicationController struct {
	Name            string
	DesiredReplicas int
	CurrentReplicas int
	Age             string
	Selector        map[string]string
	SelectorString  string
}

type Service struct {
	Name           string
	InternalIP     string
	ExternalIP     string
	Ports          []string
	Age            string
	Selector       map[string]string
	SelectorString string
}

type Endpoint struct {
	Name      string
	Age       string
	Endpoints string
}

func genNodes(list *api.NodeList) (nodes []Node) {
	for i := range list.Items {
		nodes = append(nodes, genOneNode(&list.Items[i]))
	}
	return
}

func genPods(list *api.PodList) (pods []Pod) {
	for i := range list.Items {
		pods = append(pods, genOnePod(&list.Items[i]))
	}
	return
}

func genReplicationControllers(list *api.ReplicationControllerList) (rcs []ReplicationController) {
	for i := range list.Items {
		rcs = append(rcs, genOneReplicationController(&list.Items[i]))
	}
	return
}

func genServices(list *api.ServiceList) (svcs []Service) {
	for i := range list.Items {
		svcs = append(svcs, genOneService(&list.Items[i]))
	}
	return
}

func genEndpoints(list *api.EndpointsList) (eps []Endpoint) {
	for i := range list.Items {
		eps = append(eps, genOneEndpoint(&list.Items[i]))
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

func genOneNode(node *api.Node) Node {
	conditionMap := make(map[api.NodeConditionType]*api.NodeCondition)
	NodeAllConditions := []api.NodeConditionType{api.NodeReady}
	for i := range node.Status.Conditions {
		cond := node.Status.Conditions[i]
		conditionMap[cond.Type] = &cond
	}
	var status []string
	for _, validCondition := range NodeAllConditions {
		if condition, ok := conditionMap[validCondition]; ok {
			if condition.Status == api.ConditionTrue {
				status = append(status, string(condition.Type))
			} else {
				status = append(status, "Not"+string(condition.Type))
			}
		}
	}
	if len(status) == 0 {
		status = append(status, "Unknown")
	}
	if node.Spec.Unschedulable {
		status = append(status, "SchedulingDisabled")
	}
	labels := make(map[string]string)
	for k, v := range node.Labels {
		if !strings.HasPrefix(k, "kubernetes.io") {
			labels[k] = v
		}
	}

	return Node{
		Name:   node.Name,
		Status: status,
		Age:    translateTimestamp(node.CreationTimestamp),
		Labels: labels,
	}
}

func genOneReplicationController(rc *api.ReplicationController) ReplicationController {
	result := ReplicationController{
		Name:            rc.Name,
		DesiredReplicas: rc.Spec.Replicas,
		CurrentReplicas: rc.Status.Replicas,
		Age:             translateTimestamp(rc.CreationTimestamp),
		Selector:        rc.Spec.Selector,
	}
	for k, v := range result.Selector {
		result.SelectorString += fmt.Sprintf("%s=%s,", k, v)
	}
	result.SelectorString = strings.TrimSuffix(result.SelectorString, ",")
	return result
}

func genOneService(svc *api.Service) Service {
	internalIP := svc.Spec.ClusterIP
	externalIP := getServiceExternalIP(svc)
	result := Service{
		Name:       svc.Name,
		InternalIP: internalIP,
		ExternalIP: externalIP,
		Ports:      makePorts(svc.Spec.Ports),
		Age:        translateTimestamp(svc.CreationTimestamp),
		Selector:   svc.Spec.Selector,
	}
	for k, v := range result.Selector {
		result.SelectorString += fmt.Sprintf("%s=%s,", k, v)
	}
	result.SelectorString = strings.TrimSuffix(result.SelectorString, ",")
	return result
}

func genOneEndpoint(ep *api.Endpoints) Endpoint {
	return Endpoint{
		Name:      ep.Name,
		Age:       translateTimestamp(ep.CreationTimestamp),
		Endpoints: formatEndpoints(ep, nil),
	}
}

func getServiceExternalIP(svc *api.Service) string {
	switch svc.Spec.Type {
	case api.ServiceTypeClusterIP:
		if len(svc.Spec.ExternalIPs) > 0 {
			return strings.Join(svc.Spec.ExternalIPs, ",")
		}
		return ""
	case api.ServiceTypeNodePort:
		if len(svc.Spec.ExternalIPs) > 0 {
			return strings.Join(svc.Spec.ExternalIPs, ",")
		}
		return "nodes"
	case api.ServiceTypeLoadBalancer:
		lbIps := loadBalancerStatusStringer(svc.Status.LoadBalancer)
		if len(svc.Spec.ExternalIPs) > 0 {
			result := append(strings.Split(lbIps, ","), svc.Spec.ExternalIPs...)
			return strings.Join(result, ",")
		}
		return lbIps
	}
	return "unknown"
}

// loadBalancerStatusStringer behaves just like a string interface and converts the given status to a string.
func loadBalancerStatusStringer(s api.LoadBalancerStatus) string {
	ingress := s.Ingress
	result := []string{}
	for i := range ingress {
		if ingress[i].IP != "" {
			result = append(result, ingress[i].IP)
		}
	}
	return strings.Join(result, ",")
}

func makePorts(ports []api.ServicePort) []string {
	pieces := make([]string, len(ports))
	for ix := range ports {
		port := &ports[ix]
		pieces[ix] = fmt.Sprintf("%d/%s", port.Port, port.Protocol)
	}
	return pieces
}

// Pass ports=nil for all ports.
func formatEndpoints(endpoints *api.Endpoints, ports sets.String) string {
	if len(endpoints.Subsets) == 0 {
		return ""
	}
	list := []string{}
	max := 3
	more := false
	count := 0
	for i := range endpoints.Subsets {
		ss := &endpoints.Subsets[i]
		for i := range ss.Ports {
			port := &ss.Ports[i]
			if ports == nil || ports.Has(port.Name) {
				for i := range ss.Addresses {
					if len(list) == max {
						more = true
					}
					addr := &ss.Addresses[i]
					if !more {
						list = append(list, fmt.Sprintf("%s:%d", addr.IP, port.Port))
					}
					count++
				}
			}
		}
	}
	ret := strings.Join(list, ",")
	if more {
		return fmt.Sprintf("%s + %d more...", ret, count-max)
	}
	return ret
}
