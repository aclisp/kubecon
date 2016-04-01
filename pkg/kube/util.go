package kube

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/util/sets"

	api_uv "k8s.io/kubernetes/pkg/api/unversioned"
)

func FilterEventsFromNode(events []api.Event, node *api.Node) (result []api.Event) {
	for _, ev := range events {
		if ev.Source.Host != node.Name {
			continue
		}
		if ev.InvolvedObject.Kind == "Node" {
			continue
		}
		result = append(result, ev)
	}
	return
}

func FilterNodePods(pods []*api.Pod, node *api.Node) (result []*api.Pod) {
	for _, pod := range pods {
		if pod.Spec.NodeName != node.Name {
			continue
		}
		result = append(result, pod)
	}
	return
}

func FilterTerminatedPods(pods []*api.Pod) (terminated []*api.Pod, nonTerminated []*api.Pod) {
	for _, pod := range pods {
		if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed {
			terminated = append(terminated, pod)
		} else {
			nonTerminated = append(nonTerminated, pod)
		}
	}
	return
}

func GetPodsTotalRequestsAndLimits(pods []*api.Pod) (reqs map[api.ResourceName]resource.Quantity, limits map[api.ResourceName]resource.Quantity, err error) {
	reqs, limits = map[api.ResourceName]resource.Quantity{}, map[api.ResourceName]resource.Quantity{}
	for _, pod := range pods {
		podReqs, podLimits, err := GetSinglePodTotalRequestsAndLimits(pod)
		if err != nil {
			return nil, nil, err
		}
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = *podReqValue.Copy()
			} else if err = value.Add(podReqValue); err != nil {
				return nil, nil, err
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = *podLimitValue.Copy()
			} else if err = value.Add(podLimitValue); err != nil {
				return nil, nil, err
			}
		}
	}
	return
}

func GetSinglePodTotalRequestsAndLimits(pod *api.Pod) (reqs map[api.ResourceName]resource.Quantity, limits map[api.ResourceName]resource.Quantity, err error) {
	reqs, limits = map[api.ResourceName]resource.Quantity{}, map[api.ResourceName]resource.Quantity{}
	for _, container := range pod.Spec.Containers {
		for name, quantity := range container.Resources.Requests {
			if value, ok := reqs[name]; !ok {
				reqs[name] = *quantity.Copy()
			} else if err = value.Add(quantity); err != nil {
				return nil, nil, err
			}
		}
		for name, quantity := range container.Resources.Limits {
			if value, ok := limits[name]; !ok {
				limits[name] = *quantity.Copy()
			} else if err = value.Add(quantity); err != nil {
				return nil, nil, err
			}
		}
	}
	return
}

// translateTimestamp returns the elapsed time since timestamp in
// human-readable approximation.
func TranslateTimestamp(timestamp api_uv.Time) string {
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

func TranslateResourseList(resourceList api.ResourceList) map[string]string {
	result := make(map[string]string)
	for k, v := range resourceList {
		result[string(k)] = v.String()
	}
	return result
}

func GetServiceExternalIP(svc *api.Service) string {
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

func MakePorts(ports []api.ServicePort) []string {
	pieces := make([]string, len(ports))
	for ix := range ports {
		port := &ports[ix]
		pieces[ix] = fmt.Sprintf("%d/%s", port.Port, port.Protocol)
		pieces[ix] = strings.TrimSuffix(pieces[ix], "/TCP")
	}
	return pieces
}

// Pass ports=nil for all ports.
func FormatEndpoints(endpoints *api.Endpoints, ports sets.String) string {
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
