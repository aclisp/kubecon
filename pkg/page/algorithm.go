package page

import (
	"sort"
	"strings"

	"github.com/blang/semver"
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
type byImageName []PodImage

func (a byImageName) Len() int           { return len(a) }
func (a byImageName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byImageName) Less(i, j int) bool { return a[i].Image < a[j].Image }

type combinedVersions []CombinedVersion

func (a combinedVersions) Len() int      { return len(a) }
func (a combinedVersions) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a combinedVersions) Less(i, j int) bool {
	if a[i].Version.EQ(a[j].Version) {
		return a[i].Prefix < a[j].Prefix
	}
	return a[i].Version.LT(a[j].Version)
}

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
	sort.Sort(byImageName(images))
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

// ParseImageTag parses the docker image tag string and returns a validated Version or error
func ParseImageTag(s string) (CombinedVersion, error) {
	// try parse as a SEMVER
	prefix := ""
	version, err := semver.Parse(s)
	if err == nil {
		return CombinedVersion{Prefix: prefix, Version: version}, nil
	}
	// accept format such as `vSEMVER`
	if vIndex := strings.IndexRune(s, 'v'); vIndex == 0 {
		prefix = s[:vIndex+1]
		version, err = semver.Parse(s[vIndex+1:])
		if err == nil {
			return CombinedVersion{Prefix: prefix, Version: version}, nil
		}
	}
	// accept format such as `ANY-SEMVER`
	if firstHyphenIndex := strings.IndexRune(s, '-'); firstHyphenIndex != -1 {
		prefix = s[:firstHyphenIndex+1]
		version, err = semver.Parse(s[firstHyphenIndex+1:])
		if err == nil {
			return CombinedVersion{Prefix: prefix, Version: version}, nil
		}
	}
	return CombinedVersion{}, err
}

// SortCombinedVersions sorts a slice of combined versions
func SortCombinedVersions(versions []CombinedVersion) {
	sort.Sort(combinedVersions(versions))
}

func (v CombinedVersion) String() string {
	return v.Prefix + v.Version.String()
}

func SearchCombinedVersions(items []CombinedVersion, one string) int {
	for i := range items {
		if items[i].String() == one {
			return i
		}
	}
	return -1
}

func ReverseCombinedVersions(items []CombinedVersion) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func LimitCombinedVersions(items []CombinedVersion, max int) []CombinedVersion {
	if len(items) > max {
		return items[:max]
	} else {
		return items
	}
}

func CombinedVersionsToStrings(items []CombinedVersion) (result []string) {
	for i := range items {
		result = append(result, items[i].String())
	}
	return
}
