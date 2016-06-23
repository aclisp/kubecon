package page

import (
	"sort"
)

// ByBirth implements sort.Interface for []Pod based on the ContainerBirth field.
type ByBirth []Pod

func (a ByBirth) Len() int           { return len(a) }
func (a ByBirth) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByBirth) Less(i, j int) bool { return a[i].ContainerBirth.Before(a[j].ContainerBirth) }

type ByName []Pod

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// ByImageName implements sort.Interface for []PodImage based on the Image field.
type ByImageName []PodImage

func (a ByImageName) Len() int           { return len(a) }
func (a ByImageName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByImageName) Less(i, j int) bool { return a[i].Image < a[j].Image }

func GetPodsFilters(pods []Pod) (images []PodImage, statuses []string, hosts []string) {
	imageSet := make(map[PodImage]bool)
	statusSet := make(map[string]bool)
	hostSet := make(map[string]bool)
	for i := range pods {
		for j := range pods[i].Images {
			imageSet[pods[i].Images[j]] = true
		}
		statusSet[pods[i].Status] = true
		hostSet[pods[i].HostIP] = true
	}
	for k := range imageSet {
		images = append(images, k)
	}
	for k := range statusSet {
		statuses = append(statuses, k)
	}
	for k := range hostSet {
		hosts = append(hosts, k)
	}
	sort.Sort(ByImageName(images))
	sort.Strings(statuses)
	sort.Strings(hosts)
	return
}

func (this *Pod) hasImage(image PodImage) bool {
	for _, x := range this.Images {
		if x == image {
			return true
		}
	}
	return false
}

func FilterPodsByImage(pods []Pod, image PodImage) (result []Pod) {
	for _, pod := range pods {
		if pod.hasImage(image) {
			result = append(result, pod)
		}
	}
	return
}

func FilterPodsByStatus(pods []Pod, status string) (result []Pod) {
	for _, pod := range pods {
		if pod.Status == status {
			result = append(result, pod)
		}
	}
	return
}

func FilterPodsByHost(pods []Pod, host string) (result []Pod) {
	for _, pod := range pods {
		if pod.HostIP == host {
			result = append(result, pod)
		}
	}
	return
}
