package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aclisp/kubecon/pkg/kube"
	"github.com/aclisp/kubecon/pkg/kubeclient"
	"github.com/aclisp/kubecon/pkg/page"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
)

const (
	PrivateRepoPrefix = "61.160.36.122:8080/"
	PauseImage        = "sigmas/pause:0.8.0"
)

var (
	portMapping *regexp.Regexp
)

func main() {
	defer glog.Flush()

	flag.StringVar(&kubeclient.KubeConfigFile, "kubeconfig", "kubeconfig.json", "Specify the target API server")
	flag.Set("logtostderr", "true")
	flag.Parse()

	kubeclient.Init()
	portMapping = regexp.MustCompile(`PortMapping\((.*)\)`)

	r := gin.Default()
	r.Static("/js", "js")
	r.Static("/css", "css")
	r.Static("/fonts", "fonts")
	r.Static("/img", "img")
	r.LoadHTMLGlob("pages/*.html")

	a := r.Group("/", gin.BasicAuth(gin.Accounts{
		"admin":    "secretsigma",
		"bamboo":   "oobmab",
		"default":  "test123",
		"rds":      "rrddss",
		"rds-test": "rrddsstt",
	}))

	a.GET("/", overview)
	a.GET("/namespaces/:ns", listOthersInNamespace)
	a.GET("/namespaces/:ns/pods", listPodsInNamespace)
	a.GET("/namespaces/:ns/pods/:po", describePod)
	a.GET("/namespaces/:ns/pods/:po/log", readPodLog)
	a.GET("/namespaces/:ns/pods/:po/containers/:ct/log", readContainerLog)
	a.GET("/namespaces/:ns/pods/:po/edit", editPod)
	a.GET("/namespaces/:ns/replicationcontrollers/:rc/edit", editReplicationController)
	a.GET("/namespaces/:ns/services/:svc/edit", editService)
	a.GET("/namespaces/:ns/endpoints/:ep/edit", editEndpoints)
	a.GET("/nodes/:no/edit", editNode)
	a.GET("/namespaces/:ns/events", listEventsInNamespace)
	a.GET("/nodes", listNodes)
	a.GET("/nodes/:no", describeNode)
	a.GET("/help", help)
	a.GET("/config", config)

	a.GET("/namespaces/:ns/replicationcontrollers.form", showReplicationControllerForm)
	a.POST("/namespaces/:ns/replicationcontrollers", createReplicationController)

	a.GET("/namespaces/:ns/services.form", showServiceForm)
	a.POST("/namespaces/:ns/services", createService)

	a.POST("/namespaces/:ns/pods.form", showPodsForm)
	a.POST("/namespaces/:ns/pods", performPodsAction)

	a.POST("/config/update", updateConfig)
	a.POST("/namespaces/:ns/pods/:po/update", updatePod)
	a.POST("/namespaces/:ns/pods/:po/export", updateReplicationControllerWithPod)
	a.POST("/namespaces/:ns/pods/:po/import", updatePodWithReplicationController)
	a.POST("/namespaces/:ns/services/:svc/update", updateService)
	a.POST("/namespaces/:ns/services/:svc/delete", deleteService)
	a.POST("/namespaces/:ns/endpoints/:ep/update", updateEndpoints)
	a.POST("/namespaces/:ns/endpoints/:ep/delete", deleteEndpoints)
	a.POST("/namespaces/:ns/replicationcontrollers/:rc/update", updateReplicationController)
	a.POST("/namespaces/:ns/replicationcontrollers/:rc/delete", deleteReplicationController)
	a.POST("/nodes/:no/update", updateNode)
	a.POST("/nodes/:no/delete", deleteNode)

	certFile := "kubecon.crt"
	keyFile := "kubecon.key"
	alternateIPs := []net.IP{net.ParseIP("61.160.36.122")}
	alternateDNS := []string{"kubecon"}
	if err := util.GenerateSelfSignedCert("61.160.36.122", certFile, keyFile, alternateIPs, alternateDNS); err != nil {
		glog.Errorf("Unable to generate self signed cert: %v", err)
	} else {
		glog.Infof("Using self-signed cert (%s, %s)", certFile, keyFile)
	}
	r.RunTLS(":8080", certFile, keyFile)
}

func config(c *gin.Context) {
	c.HTML(http.StatusOK, "config", gin.H{
		"title":  "Sigma Config",
		"config": kubeclient.KubeConfig,
	})
}

func updateConfig(c *gin.Context) {
	if c.MustGet(gin.AuthUserKey).(string) != "admin" {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Unauthorized"})
		return
	}

	kubeclient.KubeConfig.APIServerURL = c.PostForm("inputAPIServerURL")
	kubeclient.KubeConfig.Username = c.PostForm("inputUsername")
	kubeclient.KubeConfig.Password = c.PostForm("inputPassword")
	kubeclient.SaveKubeConfig()
	kubeclient.Init()
	c.Redirect(http.StatusMovedPermanently, "/")
}

func editService(c *gin.Context) {
	namespace := c.Param("ns")
	svcname := c.Param("svc")
	_, delete := c.GetQuery("delete")

	svc, err := kubeclient.Get().Services(namespace).Get(svcname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	b, err := json.Marshal(svc)
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

	c.HTML(http.StatusOK, "serviceEdit", gin.H{
		"title":     svcname,
		"namespace": namespace,
		"objname":   svcname,
		"json":      out.String(),
		"delete":    strconv.FormatBool(delete),
	})
}

func editNode(c *gin.Context) {
	nodename := c.Param("no")
	_, delete := c.GetQuery("delete")

	node, err := kubeclient.Get().Nodes().Get(nodename)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	b, err := json.Marshal(node)
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

	c.HTML(http.StatusOK, "nodeEdit", gin.H{
		"title":   nodename,
		"objname": nodename,
		"json":    out.String(),
		"delete":  strconv.FormatBool(delete),
	})
}

func editEndpoints(c *gin.Context) {
	namespace := c.Param("ns")
	epname := c.Param("ep")
	_, delete := c.GetQuery("delete")

	ep, err := kubeclient.Get().Endpoints(namespace).Get(epname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	b, err := json.Marshal(ep)
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

	c.HTML(http.StatusOK, "endpointsEdit", gin.H{
		"title":     epname,
		"namespace": namespace,
		"objname":   epname,
		"json":      out.String(),
		"delete":    strconv.FormatBool(delete),
	})
}

func editReplicationController(c *gin.Context) {
	namespace := c.Param("ns")
	rcname := c.Param("rc")
	_, delete := c.GetQuery("delete")

	rc, err := kubeclient.Get().ReplicationControllers(namespace).Get(rcname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	b, err := json.Marshal(rc)
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

	c.HTML(http.StatusOK, "replicationControllerEdit", gin.H{
		"title":     rcname,
		"namespace": namespace,
		"objname":   rcname,
		"json":      out.String(),
		"delete":    strconv.FormatBool(delete),
	})
}

func editPod(c *gin.Context) {
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

	var containers []string
	for i := range pod.Spec.Containers {
		containers = append(containers, pod.Spec.Containers[i].Name)
	}

	c.HTML(http.StatusOK, "podEdit", gin.H{
		"title":      podname,
		"namespace":  namespace,
		"pod":        podname,
		"containers": containers,
		"json":       out.String(),
	})
}

func readLog(c *gin.Context, namespace string, podname string, containername string, previous bool) {
	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	var containers []string
	for i := range pod.Spec.Containers {
		containers = append(containers, pod.Spec.Containers[i].Name)
	}

	container := pod.Spec.Containers[0].Name
	if len(containername) > 0 {
		container = containername
	}
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
		"title":      podname,
		"namespace":  namespace,
		"pod":        podname,
		"containers": containers,
		"log":        out.String(),
		"previous":   strconv.FormatBool(logOptions.Previous),
	})
}

func readContainerLog(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")
	containername := c.Param("ct")
	_, previous := c.GetQuery("previous")

	readLog(c, namespace, podname, containername, previous)
}

func readPodLog(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")
	_, previous := c.GetQuery("previous")

	readLog(c, namespace, podname, "", previous)
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

	var containers []string
	for i := range pod.Spec.Containers {
		containers = append(containers, pod.Spec.Containers[i].Name)
	}

	c.HTML(http.StatusOK, "podDetail", gin.H{
		"title":      podname,
		"namespace":  namespace,
		"pod":        podname,
		"containers": containers,
		"json":       out.String(),
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
		Capacity:          kube.TranslateResourseList(node.Status.Capacity),
		SystemInfo:        node.Status.NodeInfo,
	}
	allPods, err := kubeclient.GetAllPods()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	d.Pods = kube.FilterNodePods(allPods, node)
	d.TerminatedPods, d.NonTerminatedPods = kube.FilterTerminatedPods(d.Pods)
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
		events = genEvents(&api.EventList{Items: kube.FilterEventsFromNode(eventList.Items, node)})
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
	reqs, limits, err := kube.GetPodsTotalRequestsAndLimits(nonTerminated)
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
	req, limit, err := kube.GetSinglePodTotalRequestsAndLimits(pod)
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

func overview(c *gin.Context) {
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

	c.HTML(http.StatusOK, "overview", gin.H{
		"title":   "Sigma Overview",
		"summary": summary,
	})
}

func listNodes(c *gin.Context) {
	if c.MustGet(gin.AuthUserKey).(string) != "admin" {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Unauthorized"})
		return
	}

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

	user := c.MustGet(gin.AuthUserKey).(string)
	if user != "admin" && user != namespace {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Unauthorized"})
		return
	}

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

	pods := genPods(list)
	images, statuses, hosts := page.GetPodsFilters(pods)

	image, ok := c.GetQuery("image")
	if ok && len(image) > 0 {
		theImage := page.PodImage{Image: image, PrivateRepo: true}
		pods = page.FilterPodsByImage(pods, theImage)
	}

	status, ok := c.GetQuery("status")
	if ok && len(status) > 0 {
		pods = page.FilterPodsByStatus(pods, status)
	}

	host, ok := c.GetQuery("host")
	if ok && len(host) > 0 {
		pods = page.FilterPodsByHost(pods, host)
	}

	sortAlgo, ok := c.GetQuery("sort")
	if ok && len(sortAlgo) > 0 {
		switch sortAlgo {
		case "ByName":
			sort.Sort(page.ByName(pods))
		case "ByBirth":
			sort.Sort(sort.Reverse(page.ByBirth(pods)))
		default:
			sort.Sort(sort.Reverse(page.ByBirth(pods)))
		}
	} else {
		sort.Sort(sort.Reverse(page.ByBirth(pods)))
	}

	c.HTML(http.StatusOK, "podList", gin.H{
		"title":     "Sigma Pods",
		"refresh":   60,
		"namespace": namespace,
		"queries": map[string]string{
			"labelSelector": labelSelectorString,
			"image":         image,
			"status":        status,
			"host":          host,
			"sort":          sortAlgo,
		},
		"pods":     pods,
		"images":   images,
		"statuses": statuses,
		"hosts":    hosts,
	})
}

func listEventsInNamespace(c *gin.Context) {
	namespace := c.Param("ns")

	user := c.MustGet(gin.AuthUserKey).(string)
	if user != "admin" && user != namespace {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Unauthorized"})
		return
	}

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

	user := c.MustGet(gin.AuthUserKey).(string)
	if user != "admin" && user != namespace {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Unauthorized"})
		return
	}

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
	nodeList, err := kubeclient.Get().Nodes().List(labels.SelectorFromSet(labels.Set{
		"project": namespace,
	}), fields.Everything())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "nsInfo", gin.H{
		"refresh": 60,
		"title":   namespace,
		"ns":      namespace,
		"rcs":     genReplicationControllers(rcList),
		"svcs":    genServices(svcList),
		"eps":     genEndpoints(epList),
		"nodes":   genNodes(nodeList),
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
	var containerBirth unversioned.Time
	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0
	reason := string(pod.Status.Phase)
	conditionMap := make(map[api.PodConditionType]*api.PodCondition)
	PodAllConditions := []api.PodConditionType{api.PodReady}
	for i := range pod.Status.Conditions {
		cond := pod.Status.Conditions[i]
		conditionMap[cond.Type] = &cond
	}
	for _, validCondition := range PodAllConditions {
		if condition, ok := conditionMap[validCondition]; ok {
			if condition.Status != api.ConditionTrue {
				reason = "Not" + string(condition.Type)
			}
		}
	}
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
			if containerBirth.Before(container.State.Running.StartedAt) {
				containerBirth = container.State.Running.StartedAt
			}
			if container.Image == PauseImage {
				reason = "Stopped"
			}
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
	req, limit, _ := kube.GetSinglePodTotalRequestsAndLimits(pod)

	return page.Pod{
		Namespace:       pod.Namespace,
		Name:            pod.Name,
		Images:          populatePodImages(pod.Spec.Containers),
		TotalContainers: totalContainers,
		ReadyContainers: readyContainers,
		Status:          reason,
		Restarts:        restarts,
		Age:             kube.TranslateTimestamp(pod.CreationTimestamp),
		ContainerAge:    kube.TranslateTimestamp(containerBirth),
		ContainerBirth:  containerBirth.Time,
		HostNetwork:     pod.Spec.HostNetwork,
		HostIP:          pod.Spec.NodeName,
		PodIP:           podIP,
		Ports:           ports,
		Requests:        kube.TranslateResourseList(req),
		Limits:          kube.TranslateResourseList(limit),
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

	pods := kube.FilterNodePods(allPods, node)
	terminated, nonTerminated := kube.FilterTerminatedPods(pods)
	allocated, _ := computeNodeResources(nonTerminated, node)

	return page.Node{
		Name:               node.Name,
		Status:             status,
		Age:                kube.TranslateTimestamp(node.CreationTimestamp),
		Labels:             labels,
		Capacity:           kube.TranslateResourseList(node.Status.Capacity),
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
		Age:             kube.TranslateTimestamp(rc.CreationTimestamp),
		Selector:        rc.Spec.Selector,
	}
	for k, v := range result.Selector {
		result.SelectorString += fmt.Sprintf("%s=%s,", k, v)
	}
	result.SelectorString = strings.TrimSuffix(result.SelectorString, ",")
	result.TemplateImages = populatePodImages(rc.Spec.Template.Spec.Containers)
	return result
}

func populatePodImages(containers []api.Container) (images []page.PodImage) {
	for _, container := range containers {
		image := page.PodImage{
			Image:       container.Image,
			PrivateRepo: false,
		}
		if strings.HasPrefix(image.Image, PrivateRepoPrefix) {
			image.Image = strings.TrimPrefix(image.Image, PrivateRepoPrefix)
			image.PrivateRepo = true
		}
		images = append(images, image)
	}
	return
}

func genOneService(svc *api.Service) page.Service {
	internalIP := svc.Spec.ClusterIP
	externalIP := kube.GetServiceExternalIP(svc)
	result := page.Service{
		Name:       svc.Name,
		InternalIP: internalIP,
		ExternalIP: externalIP,
		Ports:      kube.MakePorts(svc.Spec.Ports),
		Age:        kube.TranslateTimestamp(svc.CreationTimestamp),
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
		Age:       kube.TranslateTimestamp(ep.CreationTimestamp),
		Endpoints: kube.FormatEndpoints(ep, nil),
	}
}

func genOneEvent(ev *api.Event) page.Event {
	return page.Event{
		FirstSeen:     kube.TranslateTimestamp(ev.FirstTimestamp),
		LastSeen:      kube.TranslateTimestamp(ev.LastTimestamp),
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

func showPodsForm(c *gin.Context) {
	namespace := c.Param("ns")
	action := c.PostForm("action")
	podsJson := c.PostForm("pods")
	location := c.PostForm("location")

	var pods []page.SimplePod
	err := json.Unmarshal([]byte(podsJson), &pods)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	var images []page.SimpleImage
	for i, image := range pods[0].Images {
		if i > 1 {
			break
		}
		splits := strings.SplitN(image, ":", 2)
		name := splits[0]
		tag := "latest"
		if len(splits) > 1 {
			tag = splits[1]
		}
		var tagList []page.CombinedVersion
		switch action {
		case "upgrade":
			tagList = getImageTags(name)
			page.SortCombinedVersions(tagList)
			index := page.SearchCombinedVersions(tagList, tag)
			tagList = tagList[index+1:]
		case "downgrade":
			tagList = getImageTags(name)
			page.SortCombinedVersions(tagList)
			index := page.SearchCombinedVersions(tagList, tag)
			if index == -1 {
				index = len(tagList)
			}
			tagList = tagList[:index]
			page.ReverseCombinedVersions(tagList)
		default:
			tagList = nil
		}
		tagList = page.LimitCombinedVersions(tagList, 50)
		images = append(images, page.SimpleImage{
			Name: name,
			Tags: page.CombinedVersionsToStrings(tagList),
		})
	}

	c.HTML(http.StatusOK, "podForm", gin.H{
		"title":     action,
		"action":    action,
		"namespace": namespace,
		"pods":      pods,
		"location":  location,
		"images":    images,
	})
}

type TagList struct {
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

func getImageTags(name string) (tags []page.CombinedVersion) {
	url := "http://" + PrivateRepoPrefix + "v2/" + name + "/tags/list"
	res, err := http.Get(url)
	if err != nil {
		glog.Errorf("Can not get image %q tags: %v", name, err)
		return nil
	}
	defer res.Body.Close()

	var data TagList
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&data)
	if err != nil {
		glog.Errorf("Can not get image %q tags: %v", name, err)
		return nil
	}
	for _, t := range data.Tags {
		v, err := page.ParseImageTag(t)
		if err != nil {
			glog.Warningf("Can not parse image %q tag %q: %v", name, t, err)
			continue
		}
		tags = append(tags, v)
	}
	return
}

func performPodsAction(c *gin.Context) {
	namespace := c.Param("ns")
	action := c.PostForm("action")
	podsJson := c.PostForm("pods")
	imagesJson := c.PostForm("images")
	checksJson := c.PostForm("checks")
	location := c.PostForm("location")

	var pods []string
	var images []string
	if err := json.Unmarshal([]byte(podsJson), &pods); err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	if err := json.Unmarshal([]byte(imagesJson), &images); err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	var fullImages []string
	for _, image := range images {
		if image == "" {
			fullImages = append(fullImages, "")
		} else {
			fullImages = append(fullImages, PrivateRepoPrefix+image)
		}
	}
	var checks []bool
	if err := json.Unmarshal([]byte(checksJson), &checks); err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	if len(checks) == 0 {
		checks = []bool{true}
	}

	var errs []error
	switch action {
	case "upgrade", "downgrade":
		for _, podname := range pods {
			if err := setPodImage(namespace, podname, fullImages); err != nil {
				errs = append(errs, err)
			}
		}
	case "start":
		for _, podname := range pods {
			if err := startPod(namespace, podname, checks); err != nil {
				errs = append(errs, err)
			}
		}
	case "stop":
		for _, podname := range pods {
			if err := stopPod(namespace, podname, checks); err != nil {
				errs = append(errs, err)
			}
		}
	case "restart":
		for _, podname := range pods {
			if err := stopPod(namespace, podname, checks); err != nil {
				errs = append(errs, err)
			}
			if err := startPod(namespace, podname, checks); err != nil {
				errs = append(errs, err)
			}
		}
	case "sync":
		for _, podname := range pods {
			if err := syncPod(namespace, podname); err != nil {
				errs = append(errs, err)
			}
		}
	case "delete":
	}

	if len(errs) > 0 {
		var errors []string
		for _, e := range errs {
			errors = append(errors, e.Error())
		}
		c.HTML(http.StatusInternalServerError, "errors", gin.H{"errors": errors})
	} else {
		re := regexp.MustCompile("status=([^&]+)")
		location = re.ReplaceAllString(location, "status=")
		re = regexp.MustCompile("image=([^&]+)")
		location = re.ReplaceAllString(location, "image=")
		c.Redirect(http.StatusMovedPermanently, location)
	}
}

func setPodImage(namespace string, podname string, fullImages []string) error {
	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		return err
	}

	for i, image := range fullImages {
		if image == "" {
			continue
		}
		glog.Infof("Set image of '%s/%s/%d': %s -> %s", namespace, podname, i, pod.Spec.Containers[i].Image, image)
		pod.Spec.Containers[i].Image = image
	}
	_, err = kubeclient.Get().Pods(namespace).Update(pod)
	if err != nil {
		return err
	}
	return nil
}

func stopPod(namespace string, podname string, checks []bool) error {
	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		return err
	}

	for i, check := range checks {
		if pod.Spec.Containers[i].Image == PauseImage {
			// Already stopped.
			continue
		}
		if check {
			paused := fmt.Sprintf("paused%d", i)
			pod.Annotations[paused] = pod.Spec.Containers[i].Image
			pod.Spec.Containers[i].Image = PauseImage
		}
	}

	_, err = kubeclient.Get().Pods(namespace).Update(pod)
	if err != nil {
		return err
	}
	return nil
}

func startPod(namespace string, podname string, checks []bool) error {
	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		return err
	}

	for i, check := range checks {
		if pod.Spec.Containers[i].Image != PauseImage {
			// Already started.
			continue
		}
		if check {
			paused := fmt.Sprintf("paused%d", i)
			pod.Spec.Containers[i].Image = pod.Annotations[paused]
			delete(pod.Annotations, paused)
		}
	}

	_, err = kubeclient.Get().Pods(namespace).Update(pod)
	if err != nil {
		return err
	}
	return nil
}

func syncPod(namespace string, podname string) error {
	pod, err := kubeclient.Get().Pods(namespace).Get(podname)
	if err != nil {
		return err
	}
	rcname, ok := pod.Labels["managed-by"]
	if !ok {
		return fmt.Errorf("Need a `managed-by` label")
	}
	rc, err := kubeclient.Get().ReplicationControllers(namespace).Get(rcname)
	if err != nil {
		return err
	}
	nodeName := pod.Spec.NodeName
	pod.Spec = rc.Spec.Template.Spec
	pod.Spec.NodeName = nodeName
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["copied-from"] = rcname
	for k, v := range rc.Spec.Template.Annotations {
		if strings.HasPrefix(k, "config/") {
			pod.Annotations[k] = v
		}
	}
	_, err = kubeclient.Get().Pods(namespace).Update(pod)
	return err
}

func updatePod(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")
	podjson := c.PostForm("json")

	var pod api.Pod
	err := json.Unmarshal([]byte(podjson), &pod)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	r, _ := kubeclient.Get().Pods(namespace).Get(pod.Name)
	pod.ResourceVersion = r.ResourceVersion
	_, err = kubeclient.Get().Pods(namespace).Update(&pod)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s/pods/%s/edit", namespace, podname))
}

func updateReplicationControllerWithPod(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")
	podjson := c.PostForm("json")

	var pod api.Pod
	err := json.Unmarshal([]byte(podjson), &pod)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	rcname, ok := pod.Labels["managed-by"]
	if !ok {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Need a `managed-by` label"})
		return
	}
	rc, err := kubeclient.Get().ReplicationControllers(namespace).Get(rcname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	nodeName := rc.Spec.Template.Spec.NodeName
	rc.Spec.Template.Spec = pod.Spec
	rc.Spec.Template.Spec.NodeName = nodeName
	if rc.Annotations == nil {
		rc.Annotations = make(map[string]string)
	}
	rc.Annotations["copied-from"] = podname
	if rc.Spec.Template.Annotations == nil {
		rc.Spec.Template.Annotations = make(map[string]string)
	}
	for k, v := range pod.Annotations {
		if strings.HasPrefix(k, "config/") {
			rc.Spec.Template.Annotations[k] = v
		}
	}
	_, err = kubeclient.Get().ReplicationControllers(namespace).Update(rc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s/pods/%s/edit", namespace, podname))
}

func updatePodWithReplicationController(c *gin.Context) {
	namespace := c.Param("ns")
	podname := c.Param("po")

	err := syncPod(namespace, podname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s/pods/%s/edit", namespace, podname))
}

func updateReplicationController(c *gin.Context) {
	namespace := c.Param("ns")
	rcname := c.Param("rc")
	rcjson := c.PostForm("json")

	var rc api.ReplicationController
	err := json.Unmarshal([]byte(rcjson), &rc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	_, err = kubeclient.Get().ReplicationControllers(namespace).Update(&rc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s/replicationcontrollers/%s/edit", namespace, rcname))
}

func deleteReplicationController(c *gin.Context) {
	namespace := c.Param("ns")
	rcname := c.Param("rc")

	rc, err := kubeclient.Get().ReplicationControllers(namespace).Get(rcname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}
	if rc.Spec.Replicas > 0 || rc.Status.Replicas > 0 {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": "Replicas must be 0"})
		return
	}
	err = kubeclient.Get().ReplicationControllers(namespace).Delete(rcname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s", namespace))
}

func showReplicationControllerForm(c *gin.Context) {
	namespace := c.Param("ns")

	bytes, err := ioutil.ReadFile("replication-controller.json")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "replicationControllerForm", gin.H{
		"title":     namespace,
		"namespace": namespace,
		"json":      string(bytes),
	})
}

func createReplicationController(c *gin.Context) {
	namespace := c.Param("ns")
	rcjson := c.PostForm("json")

	var rc api.ReplicationController
	err := json.Unmarshal([]byte(rcjson), &rc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	if rc.Spec.Selector == nil {
		rc.Spec.Selector = make(map[string]string)
	}
	rc.Spec.Selector["managed-by"] = rc.Name
	if rc.Spec.Template.Labels == nil {
		rc.Spec.Template.Labels = make(map[string]string)
	}
	rc.Spec.Template.Labels["managed-by"] = rc.Name
	rc.Spec.Template.Spec.Containers[0].Name = rc.Name

	var meta api.ObjectMeta // clean metadata
	meta.Name = rc.Name
	meta.GenerateName = rc.GenerateName
	meta.Labels = rc.Labels
	meta.Annotations = rc.Annotations
	if meta.Labels != nil {
		meta.Labels["managed-by"] = rc.Name
	}
	rc.ObjectMeta = meta

	_, err = kubeclient.Get().ReplicationControllers(namespace).Create(&rc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s", namespace))
}

func updateService(c *gin.Context) {
	namespace := c.Param("ns")
	svcname := c.Param("svc")
	svcjson := c.PostForm("json")

	var svc api.Service
	err := json.Unmarshal([]byte(svcjson), &svc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	_, err = kubeclient.Get().Services(namespace).Update(&svc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s/services/%s/edit", namespace, svcname))
}

func deleteService(c *gin.Context) {
	namespace := c.Param("ns")
	svcname := c.Param("svc")

	err := kubeclient.Get().Services(namespace).Delete(svcname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s", namespace))
}

func updateEndpoints(c *gin.Context) {
	namespace := c.Param("ns")
	epname := c.Param("ep")
	epjson := c.PostForm("json")

	var ep api.Endpoints
	err := json.Unmarshal([]byte(epjson), &ep)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	_, err = kubeclient.Get().Endpoints(namespace).Update(&ep)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s/endpoints/%s/edit", namespace, epname))
}

func deleteEndpoints(c *gin.Context) {
	namespace := c.Param("ns")
	epname := c.Param("ep")

	err := kubeclient.Get().Endpoints(namespace).Delete(epname)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s", namespace))
}

func showServiceForm(c *gin.Context) {
	namespace := c.Param("ns")

	bytes, err := ioutil.ReadFile("service.json")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "serviceForm", gin.H{
		"title":     namespace,
		"namespace": namespace,
		"json":      string(bytes),
	})
}

func createService(c *gin.Context) {
	namespace := c.Param("ns")
	svcjson := c.PostForm("json")

	var svc api.Service
	err := json.Unmarshal([]byte(svcjson), &svc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	_, err = kubeclient.Get().Services(namespace).Create(&svc)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/namespaces/%s", namespace))
}

func updateNode(c *gin.Context) {
	nodename := c.Param("no")
	nodejson := c.PostForm("json")

	var node api.Node
	err := json.Unmarshal([]byte(nodejson), &node)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	r, _ := kubeclient.Get().Nodes().Get(node.Name)
	node.ResourceVersion = r.ResourceVersion
	_, err = kubeclient.Get().Nodes().Update(&node)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/nodes/%s/edit", nodename))
}

func deleteNode(c *gin.Context) {
	nodename := c.Param("no")

	err := kubeclient.Get().Nodes().Delete(nodename)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/nodes"))
}
