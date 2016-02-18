package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aclisp/kubecon/pkg/kubeclient"
	"github.com/aclisp/kubecon/pkg/page"
	"github.com/aclisp/kubecon/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/types"
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
	r.LoadHTMLGlob("pages/*.html")

	r.GET("/", index)
	r.GET("/namespaces/:ns", listOthersInNamespace)
	r.GET("/namespaces/:ns/pods", listPodsInNamespace)
	r.GET("/namespaces/:ns/pods/:po", describePod)
	r.GET("/namespaces/:ns/pods/:po/log", readPodLog)
	r.GET("/namespaces/:ns/events", listEventsInNamespace)
	r.GET("/nodes", listNodes)
	r.GET("/nodes/:no", describeNode)

	r.GET("/help", help)

	r.Run(":8080") // listen and serve on 0.0.0.0:8080
}

func readPodLog(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")
	_, previous := c.GetQuery("previous")

	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	container := pod.Spec.Containers[0].Name

	limitBytes := int64(256 * 1024)
	tailLines := int64(500)
	logOptions := &api.PodLogOptions{
		Container:  container,
		Follow:     false,
		Previous:   previous,
		Timestamps: false,
		TailLines:  &tailLines,
		LimitBytes: &limitBytes,
	}

	req := kubeclient.Get().RESTClient.
		Get().
		Namespace(namespace).
		Name(podname).
		Resource("pods").
		SubResource("log").
		Param("container", logOptions.Container).
		Param("follow", strconv.FormatBool(logOptions.Follow)).
		Param("previous", strconv.FormatBool(logOptions.Previous)).
		Param("timestamps", strconv.FormatBool(logOptions.Timestamps)).
		Param("tailLines", strconv.FormatInt(*logOptions.TailLines, 10)).
		Param("limitBytes", strconv.FormatInt(*logOptions.LimitBytes, 10))
	readCloser, err := req.Stream()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	defer readCloser.Close()

	var out bytes.Buffer
	_, err = io.Copy(&out, readCloser)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "podLog", gin.H{
		"title":     podname,
		"namespace": namespace,
		"pod":       podname,
		"log":       out.String(),
		"previous":  strconv.FormatBool(logOptions.Previous),
	})
}

func describePod(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")

	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	b, err := json.Marshal(pod)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	var out bytes.Buffer
	err = json.Indent(&out, b, "", "  ")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "podDetail", gin.H{
		"title":     podname,
		"namespace": namespace,
		"pod":       podname,
		"json":      out.String(),
	})
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

	var nodeEvents []page.Event
	var nodeEventList *api.EventList
	if ref, err := api.GetReference(node); err != nil {
		glog.Errorf("Unable to construct reference to '%#v': %v", node, err)
	} else {
		ref.UID = types.UID(ref.Name)
		if nodeEventList, err = kubeclient.Get().Events("").Search(ref); err != nil {
			glog.Errorf("Unable to search events for '%#v': %v", node, err)
		}
	}
	if nodeEventList != nil {
		nodeEvents = genEvents(nodeEventList)
	}

	var events []page.Event
	var eventList *api.EventList
	if eventList, err = kubeclient.Get().Events("").List(labels.Everything(), fields.Everything()); err != nil {
		glog.Errorf("Unable to search events for '%#v': %v", node, err)
	}
	if eventList != nil {
		events = genEvents(&api.EventList{Items: util.FilterEventsFromNode(eventList.Items, node)})
	}

	c.HTML(http.StatusOK, "nodeDetail", gin.H{
		"title":      nodename,
		"node":       d,
		"pods":       pods,
		"events":     events,
		"nodeEvents": nodeEvents,
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

func help(c *gin.Context) {
	c.HTML(http.StatusOK, "help", gin.H{
		"title": "Sigma Help",
	})
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
		eventList, err := kubeclient.Get().Events(namespace).List(labels.Everything(), fields.Everything())
		if err != nil {
			glog.Errorf("Can not get events in namespace '%s': %v", namespace, err)
			eventList = &api.EventList{}
		}
		summary.Namespaces = append(summary.Namespaces, page.Namespace{
			Name:       namespace,
			PodCount:   len(podList.Items),
			EventCount: len(eventList.Items),
		})
	}
	nodeList, err := kubeclient.Get().Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	summary.NodeCount = len(nodeList.Items)

	c.HTML(http.StatusOK, "index", gin.H{
		"title":   "Sigma Overview",
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
		"title": "Sigma Nodes",
		"nodes": genNodes(list),
	})
}

func listPodsInNamespace(c *gin.Context) {
	namespace := c.Param("ns")
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

	list, err := kubeclient.Get().Pods(namespace).List(labelSelector, fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "podList", gin.H{
		"title": "Sigma Pods",
		"pods":  genPods(list),
	})
}

func listEventsInNamespace(c *gin.Context) {
	namespace := c.Param("ns")

	list, err := kubeclient.Get().Events(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "eventList", gin.H{
		"title":  "Sigma Events",
		"events": genEvents(list),
	})
}

func listOthersInNamespace(c *gin.Context) {
	namespace := c.Param("ns")
	rcList, err := kubeclient.Get().ReplicationControllers(namespace).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	svcList, err := kubeclient.Get().Services(namespace).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	epList, err := kubeclient.Get().Endpoints(namespace).List(labels.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "nsInfo", gin.H{
		"title": namespace,
		"ns":    namespace,
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

func genEvents(list *api.EventList) (events []page.Event) {
	sort.Sort(sort.Reverse(kubectl.SortableEvents(list.Items)))
	for i := range list.Items {
		events = append(events, genOneEvent(&list.Items[i]))
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
		matches := portMapping.FindStringSubmatch(pod.Status.Message)
		if len(matches) > 1 {
			portString = matches[1]
		}
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
		Namespace:       pod.Namespace,
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

func genOneEvent(ev *api.Event) page.Event {
	return page.Event{
		FirstSeen:     util.TranslateTimestamp(ev.FirstTimestamp),
		LastSeen:      util.TranslateTimestamp(ev.LastTimestamp),
		Count:         ev.Count,
		FromComponent: ev.Source.Component,
		FromHost:      ev.Source.Host,
		SubobjectName: ev.InvolvedObject.Name,
		SubobjectKind: ev.InvolvedObject.Kind,
		SubobjectPath: ev.InvolvedObject.FieldPath,
		Reason:        ev.Reason,
		Message:       ev.Message,
	}
}
