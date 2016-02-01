package main

import (
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aclisp/kubecon/pkg/kubeclient"
	"github.com/aclisp/kubecon/pkg/page"
	"github.com/aclisp/kubecon/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

const (
	PrivateRepoPrefix = "61.160.36.122:8080/"
)

var (
	portMapping *regexp.Regexp
)

func main() {
	defer glog.Flush()

	flag.Set("logtostderr", "true")
	flag.Parse()

	kubeclient.Init()
	portMapping = regexp.MustCompile(`PortMapping\((.*)\)`)

	r := gin.Default()
	r.Static("/js", "js")
	r.Static("/css", "css")
	r.Static("/fonts", "fonts")
	r.LoadHTMLGlob("pages/*")

	r.GET("/", index)
	r.GET("/namespaces/:ns", listOthersInNamespace)
	r.GET("/namespaces/:ns/pods", listPodsInNamespace)
	r.GET("/nodes", listNodes)
	r.GET("/nodes/:no", describeNode)

	r.Run(":8080") // listen and serve on 0.0.0.0:8080
}

func describeNode(c *gin.Context) {
	nodename := c.Param("no")

	node, err := kubeclient.Get().Nodes().Get(nodename)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	d := page.NodeDetail{
		Name:              node.Name,
		Labels:            node.Labels,
		CreationTimestamp: node.CreationTimestamp.Time.Format(time.RFC1123Z),
		Conditions:        node.Status.Conditions,
		Capacity:          util.TranslateResourseList(node.Status.Capacity),
		SystemInfo:        node.Status.NodeInfo,
	}
	allPods, err := kubeclient.GetAllPods()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	d.Pods = util.FilterNodePods(allPods, node)
	d.TerminatedPods, d.NonTerminatedPods = util.FilterTerminatedPods(d.Pods)
	d.NonTerminatedPodsResources = computePodsResources(d.NonTerminatedPods, node)
	d.TerminatedPodsResources = computePodsResources(d.TerminatedPods, node)
	d.AllocatedResources, err = computeNodeResources(d.NonTerminatedPods, node)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	var pods []page.Pod
	for _, pod := range d.Pods {
		pods = append(pods, genOnePod(pod))
	}

	c.HTML(http.StatusOK, "nodeDetail", gin.H{
		"title":  "Node",
		"detail": d,
		"pods":   pods,
	})
}

func computeNodeResources(nonTerminated []*api.Pod, node *api.Node) (page.Resources, error) {
	reqs, limits, err := util.GetPodsTotalRequestsAndLimits(nonTerminated)
	if err != nil {
		return page.Resources{}, err
	}
	cpuReqs, cpuLimits, memoryReqs, memoryLimits := reqs[api.ResourceCPU], limits[api.ResourceCPU], reqs[api.ResourceMemory], limits[api.ResourceMemory]
	fractionCpuReqs := float64(cpuReqs.MilliValue()) / float64(node.Status.Capacity.Cpu().MilliValue()) * 100
	fractionCpuLimits := float64(cpuLimits.MilliValue()) / float64(node.Status.Capacity.Cpu().MilliValue()) * 100
	fractionMemoryReqs := float64(memoryReqs.MilliValue()) / float64(node.Status.Capacity.Memory().MilliValue()) * 100
	fractionMemoryLimits := float64(memoryLimits.MilliValue()) / float64(node.Status.Capacity.Memory().MilliValue()) * 100
	return page.Resources{
		CpuRequest:            cpuReqs.String(),
		CpuLimit:              cpuLimits.String(),
		MemoryRequest:         memoryReqs.String(),
		MemoryLimit:           memoryLimits.String(),
		FractionCpuRequest:    int64(fractionCpuReqs),
		FractionCpuLimit:      int64(fractionCpuLimits),
		FractionMemoryRequest: int64(fractionMemoryReqs),
		FractionMemoryLimit:   int64(fractionMemoryLimits),
	}, nil
}

func computePodsResources(pods []*api.Pod, node *api.Node) (result []page.Resources) {
	for _, pod := range pods {
		m, err := computePodResources(pod, node)
		if err != nil {
			glog.Errorf("Ignore pod '%s/%s' resources: %v", pod.Namespace, pod.Name, err)
		}
		result = append(result, m)
	}
	return
}

func computePodResources(pod *api.Pod, node *api.Node) (page.Resources, error) {
	req, limit, err := util.GetSinglePodTotalRequestsAndLimits(pod)
	if err != nil {
		return page.Resources{}, err
	}
	cpuReq, cpuLimit, memoryReq, memoryLimit := req[api.ResourceCPU], limit[api.ResourceCPU], req[api.ResourceMemory], limit[api.ResourceMemory]
	fractionCpuReq := float64(cpuReq.MilliValue()) / float64(node.Status.Capacity.Cpu().MilliValue()) * 100
	fractionCpuLimit := float64(cpuLimit.MilliValue()) / float64(node.Status.Capacity.Cpu().MilliValue()) * 100
	fractionMemoryReq := float64(memoryReq.MilliValue()) / float64(node.Status.Capacity.Memory().MilliValue()) * 100
	fractionMemoryLimit := float64(memoryLimit.MilliValue()) / float64(node.Status.Capacity.Memory().MilliValue()) * 100
	return page.Resources{
		Namespace:             pod.Namespace,
		Name:                  pod.Name,
		CpuRequest:            cpuReq.String(),
		CpuLimit:              cpuLimit.String(),
		MemoryRequest:         memoryReq.String(),
		MemoryLimit:           memoryLimit.String(),
		FractionCpuRequest:    int64(fractionCpuReq),
		FractionCpuLimit:      int64(fractionCpuLimit),
		FractionMemoryRequest: int64(fractionMemoryReq),
		FractionMemoryLimit:   int64(fractionMemoryLimit),
	}, nil
}

func index(c *gin.Context) {
	namespaces, err := kubeclient.Get().Namespaces().List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	summary := page.Summary{}
	for i := range namespaces.Items {
		namespace := namespaces.Items[i].Name
		podList, err := kubeclient.Get().Pods(namespace).List(labels.Everything(), fields.Everything())
		if err != nil {
			glog.Errorf("Can not get pods in namespace '%s': %v", namespace, err)
			continue
		}
		summary.Namespaces = append(summary.Namespaces, page.Namespace{
			Name:     namespace,
			PodCount: len(podList.Items),
		})
	}
	nodeList, err := kubeclient.Get().Nodes().List(labels.Everything(), fields.Everything())
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
	list, err := kubeclient.Get().Nodes().List(labels.Everything(), fields.Everything())
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

	list, err := kubeclient.Get().Pods(ns).List(labelSelector, fields.Everything())
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
	rcList, err := kubeclient.Get().ReplicationControllers(ns).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	svcList, err := kubeclient.Get().Services(ns).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	epList, err := kubeclient.Get().Endpoints(ns).List(labels.Everything())
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

func genNodes(list *api.NodeList) (nodes []page.Node) {
	allPods, _ := kubeclient.GetAllPods()
	for i := range list.Items {
		nodes = append(nodes, genOneNode(&list.Items[i], allPods))
	}
	return
}

func genPods(list *api.PodList) (pods []page.Pod) {
	for i := range list.Items {
		pods = append(pods, genOnePod(&list.Items[i]))
	}
	return
}

func genReplicationControllers(list *api.ReplicationControllerList) (rcs []page.ReplicationController) {
	for i := range list.Items {
		rcs = append(rcs, genOneReplicationController(&list.Items[i]))
	}
	return
}

func genServices(list *api.ServiceList) (svcs []page.Service) {
	for i := range list.Items {
		if list.Items[i].Name == "kubernetes" {
			continue
		}
		svcs = append(svcs, genOneService(&list.Items[i]))
	}
	return
}

func genEndpoints(list *api.EndpointsList) (eps []page.Endpoint) {
	for i := range list.Items {
		if list.Items[i].Name == "kubernetes" {
			continue
		}
		eps = append(eps, genOneEndpoint(&list.Items[i]))
	}
	return
}

func genOnePod(pod *api.Pod) page.Pod {
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
	portString := ""
	if pod.Spec.HostNetwork {
		podIP = ""
		for i := range pod.Spec.Containers {
			for j := range pod.Spec.Containers[i].Ports {
				port := pod.Spec.Containers[i].Ports[j]
				portString += fmt.Sprintf("%d/%s,", port.HostPort, port.Protocol)
			}
		}
		portString = strings.TrimSuffix(portString, ",")
	} else {
		podIP = pod.Status.PodIP
		portString = portMapping.FindStringSubmatch(pod.Status.Message)[1]
	}
	var ports []string
	for _, p := range strings.Split(portString, ",") {
		ports = append(ports, strings.TrimSuffix(p, "/TCP"))
	}
	image := pod.Spec.Containers[0].Image
	privateRepo := false
	if strings.HasPrefix(image, PrivateRepoPrefix) {
		image = strings.TrimPrefix(image, PrivateRepoPrefix)
		privateRepo = true
	}
	req, limit, _ := util.GetSinglePodTotalRequestsAndLimits(pod)

	return page.Pod{
		Name:            pod.Name,
		Image:           image,
		PrivateRepo:     privateRepo,
		TotalContainers: totalContainers,
		ReadyContainers: readyContainers,
		Status:          reason,
		Restarts:        restarts,
		Age:             util.TranslateTimestamp(pod.CreationTimestamp),
		HostNetwork:     pod.Spec.HostNetwork,
		HostIP:          pod.Spec.NodeName,
		PodIP:           podIP,
		Ports:           ports,
		Requests:        util.TranslateResourseList(req),
		Limits:          util.TranslateResourseList(limit),
	}
}

func genOneNode(node *api.Node, allPods []*api.Pod) page.Node {
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

	pods := util.FilterNodePods(allPods, node)
	terminated, nonTerminated := util.FilterTerminatedPods(pods)
	allocated, _ := computeNodeResources(nonTerminated, node)

	return page.Node{
		Name:               node.Name,
		Status:             status,
		Age:                util.TranslateTimestamp(node.CreationTimestamp),
		Labels:             labels,
		Capacity:           util.TranslateResourseList(node.Status.Capacity),
		Pods:               pods,
		TerminatedPods:     terminated,
		NonTerminatedPods:  nonTerminated,
		AllocatedResources: allocated,
		FractionPods:       int64(float64(len(pods)) / float64(node.Status.Capacity.Pods().Value()) * 100),
	}
}

func genOneReplicationController(rc *api.ReplicationController) page.ReplicationController {
	result := page.ReplicationController{
		Name:            rc.Name,
		DesiredReplicas: rc.Spec.Replicas,
		CurrentReplicas: rc.Status.Replicas,
		Age:             util.TranslateTimestamp(rc.CreationTimestamp),
		Selector:        rc.Spec.Selector,
	}
	for k, v := range result.Selector {
		result.SelectorString += fmt.Sprintf("%s=%s,", k, v)
	}
	result.SelectorString = strings.TrimSuffix(result.SelectorString, ",")
	return result
}

func genOneService(svc *api.Service) page.Service {
	internalIP := svc.Spec.ClusterIP
	externalIP := util.GetServiceExternalIP(svc)
	result := page.Service{
		Name:       svc.Name,
		InternalIP: internalIP,
		ExternalIP: externalIP,
		Ports:      util.MakePorts(svc.Spec.Ports),
		Age:        util.TranslateTimestamp(svc.CreationTimestamp),
		Selector:   svc.Spec.Selector,
	}
	for k, v := range result.Selector {
		result.SelectorString += fmt.Sprintf("%s=%s,", k, v)
	}
	result.SelectorString = strings.TrimSuffix(result.SelectorString, ",")
	return result
}

func genOneEndpoint(ep *api.Endpoints) page.Endpoint {
	return page.Endpoint{
		Name:      ep.Name,
		Age:       util.TranslateTimestamp(ep.CreationTimestamp),
		Endpoints: util.FormatEndpoints(ep, nil),
	}
}
