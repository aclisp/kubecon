package page

import (
	"k8s.io/kubernetes/pkg/api"
)

type NodeDetail struct {
	Name                       string
	Labels                     map[string]string
	CreationTimestamp          string
	Conditions                 []api.NodeCondition
	Capacity                   map[string]string
	SystemInfo                 api.NodeSystemInfo
	Pods                       []*api.Pod
	TerminatedPods             []*api.Pod
	NonTerminatedPods          []*api.Pod
	TerminatedPodsResources    []Resources
	NonTerminatedPodsResources []Resources
	AllocatedResources         Resources
}

type Event struct {
	FirstSeen     string
	LastSeen      string
	Count         int
	FromComponent string
	FromHost      string
	SubobjectName string
	SubobjectKind string
	SubobjectPath string
	Reason        string
	Message       string
}

type Resources struct {
	Namespace             string
	Name                  string
	CpuRequest            string
	CpuLimit              string
	MemoryRequest         string
	MemoryLimit           string
	FractionCpuRequest    int64
	FractionCpuLimit      int64
	FractionMemoryRequest int64
	FractionMemoryLimit   int64
}

type PodImage struct {
	Image       string
	PrivateRepo bool
}

type Pod struct {
	Namespace       string
	Name            string
	Images          []PodImage
	TotalContainers int
	ReadyContainers int
	Status          string
	Restarts        int
	Age             string
	HostNetwork     bool
	HostIP          string
	PodIP           string
	Ports           []string
	Requests        map[string]string
	Limits          map[string]string
}

type Node struct {
	Name               string
	Status             []string
	Age                string
	Labels             map[string]string
	Capacity           map[string]string
	Pods               []*api.Pod
	TerminatedPods     []*api.Pod
	NonTerminatedPods  []*api.Pod
	AllocatedResources Resources
	FractionPods       int64
}

type Namespace struct {
	Name       string
	PodCount   int
	EventCount int
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
