package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type namespaceList []string

func (nl *namespaceList) String() string { return strings.Join(*nl, ",") }
func (nl *namespaceList) Set(value string) error {
	*nl = append(*nl, value)
	return nil
}

type ResourceFilter struct {
	Before       time.Time
	After        time.Time
	Start        time.Time
	End          time.Time
	OutputDir    string
	ResourceData bool
}

func init() {
	flag.Usage = func() {
		cmd := filepath.Base(os.Args[0])
		if strings.HasPrefix(cmd, "kubectl") {
			cmd = strings.Replace(cmd, "-", " ", 1)
			cmd = strings.Replace(cmd, "_", "-", 1)
		}

		example := func(args string) string {
			if args == "" {
				return cmd
			}
			return fmt.Sprintf("%s %s", cmd, args)
		}

		message := "Get resources from the K8s/OpenShift cluster. Note: all flags are optional.\n\n" + "Flags:\n"

		fmt.Fprintf(flag.CommandLine.Output(), "%s", message)
		flag.PrintDefaults()

		fmt.Fprintln(flag.CommandLine.Output(), `
Examples:
  Get all resources (namespaced + cluster resources)
  `+example("")+`

  Get only cluster-scoped resources
  `+example(`--namespace=""`)+`

  Get only all namespaced resources
  `+example(`--namespace="*" --exclude-cluster-resources=true`)+`

  Get specific namespace resources
  `+example(`--namespace=default`)+`

  Get multiple namespace resources
  `+example(`--namespace=default --namespace=sample-namespace`)+`

  Get all resources created before a given time
  `+example(`--before=2025-08-10T09:39:09Z`)+`

  Get all resources created after a given time
  `+example(`--after=2025-08-10T09:39:09Z`)+`

  Get all resources between two times
  `+example(`--start=2025-08-10T09:39:09Z --end=2025-08-10T10:30:02Z`)+`

  Get 'default' namespace resources after a given time
  `+example(`--namespace=default --after=2025-08-10T09:39:09Z`)+`

  Get resource details added in CSV output
  `+example(`--namespace=default --after=2025-08-10T09:39:09Z --resource-data=true`)+`

  Save all output YAMLs to a directory
  `+example(`--output=<Your directory name>`)+`

  Save 'default' namespace resources in directory 'default_namespace_resources'
  `+example(`--namespace=default --output=default_namespace_resources`)+`

  Notes:
  (1) Flags --resource-data and --output are mutually exclusive
  (2) Exclude specific group(s) from retrieval by listing them in the hidden file .get-resources-excluded-groups in user's HOME directory.
      Each group should be written on a separate line. Lines starting with a hash (#) are treated as comments and ignored.
      Commonly excluded groups are:
      $ cat ~/.get-resources-excluded-groups
        events.k8s.io
        metrics.k8s.io
        image.openshift.io
        packages.operators.coreos.com
`)
	}
}

func main() {
	var namespaces namespaceList
	var excludeCluster, resourceData bool
	var beforeStr, afterStr, startStr, endStr, outputDir string

	flag.Var(&namespaces, "namespace", "Namespace(s) to process. Use '*' for all, '' for only cluster resources.")
	flag.BoolVar(&excludeCluster, "exclude-cluster-resources", false, "Exclude cluster-scoped resources")
	flag.StringVar(&beforeStr, "before", "", "Only include resources created before this RFC3339 timestamp")
	flag.StringVar(&afterStr, "after", "", "Only include resources created after this RFC3339 timestamp")
	flag.StringVar(&startStr, "start", "", "Start time for filtering resources (use with --end)")
	flag.StringVar(&endStr, "end", "", "End time for filtering resources (use with --start)")
	flag.StringVar(&outputDir, "output", "", "Directory to save collected resource YAMLs")
	flag.BoolVar(&resourceData, "resource-data", false, "Add resource details in CSV output")

	flag.Parse()

	filter, err := validateAndBuildFilter(beforeStr, afterStr, startStr, endStr, outputDir, resourceData)
	if err != nil {
		log.Fatalf("Flag validation error: %v", err)
	}

	// Init K8s clients
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to load kubeconfig: %v", err)
	}
	dynClient, _ := dynamic.NewForConfig(config)
	discClient, _ := discovery.NewDiscoveryClientForConfig(config)

	if resourceData {
		fmt.Println("kind,plural,apiversion,namespace,name,creationtimestamp,data")
	} else if outputDir == "" {
		fmt.Println("kind,plural,apiversion,namespace,name,creationtimestamp")
	}

	// Decision logic
	switch {
	case len(namespaces) == 0:
		if excludeCluster {
			fmt.Println("Nothing to process: no namespaces and cluster excluded")
			os.Exit(0)
		}
		processAllResources(dynClient, discClient, filter)

	case len(namespaces) == 1 && namespaces[0] == "":
		processOnlyClusterResources(dynClient, discClient, filter)

	case contains(namespaces, "*"):
		if excludeCluster {
			processOnlyNamespaces(dynClient, discClient, filter, []string{"*"})
		} else {
			processAllResources(dynClient, discClient, filter)
		}

	default:
		if excludeCluster {
			processOnlyNamespaces(dynClient, discClient, filter, namespaces)
		} else {
			processNamespacesAndCluster(dynClient, discClient, filter, namespaces)
		}
	}
}

// Validation and Filtering
func validateAndBuildFilter(beforeStr, afterStr, startStr, endStr, output string, resourceData bool) (ResourceFilter, error) {
	var filter ResourceFilter
	var err error

	if beforeStr != "" && afterStr != "" {
		return filter, errors.New("cannot use both --before and --after")
	}
	if (startStr != "" && endStr == "") || (startStr == "" && endStr != "") {
		return filter, errors.New("--start and --end must be used together")
	}
	if (beforeStr != "" || afterStr != "") && (startStr != "" || endStr != "") {
		return filter, errors.New("--before/--after cannot be used with --start/--end")
	}

	if beforeStr != "" {
		filter.Before, err = time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return filter, fmt.Errorf("invalid --before timestamp: %v", err)
		}
	}
	if afterStr != "" {
		filter.After, err = time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return filter, fmt.Errorf("invalid --after timestamp: %v", err)
		}
	}
	if startStr != "" {
		filter.Start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return filter, fmt.Errorf("invalid --start timestamp: %v", err)
		}
		filter.End, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return filter, fmt.Errorf("invalid --end timestamp: %v", err)
		}
	}

	if resourceData && output != "" {
		return filter, errors.New("--resource-data and --output are mutually exclusive")
	}

	filter.OutputDir = output
	filter.ResourceData = resourceData
	return filter, nil
}

func contains(list []string, val string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

// Different Processing functions
func processAllResources(dyn dynamic.Interface, disc *discovery.DiscoveryClient, filter ResourceFilter) {
	processResources(dyn, disc, filter, nil, true, true) // nil = all namespaces
}

func processNamespacesAndCluster(dyn dynamic.Interface, disc *discovery.DiscoveryClient, filter ResourceFilter, namespaces []string) {
	processResources(dyn, disc, filter, namespaces, true, true)
}

func processOnlyNamespaces(dyn dynamic.Interface, disc *discovery.DiscoveryClient, filter ResourceFilter, namespaces []string) {
	processResources(dyn, disc, filter, namespaces, false, true)
}

func processOnlyClusterResources(dyn dynamic.Interface, disc *discovery.DiscoveryClient, filter ResourceFilter) {
	processResources(dyn, disc, filter, nil, true, false)
}

func getExcludedGroups(filename string) map[string]bool {
	excludedGroups := make(map[string]bool)
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: can't get user home directory: %v", err)
		return excludedGroups
	}

	filepath := filepath.Join(home, filename)
	f, err := os.Open(filepath)
	if err != nil {
		// File not found, return empty map
		return excludedGroups
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		excludedGroups[line] = true
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Warning: error reading excluded file %s: %v", filepath, err)
	}
	return excludedGroups
}

func processResources(dyn dynamic.Interface, disc *discovery.DiscoveryClient, filter ResourceFilter, namespaces []string, includeCluster bool, processNamespacedResources bool) {
	// Discover resources
	apiResources, err := disc.ServerPreferredResources()
	if err != nil {
		log.Fatalf("Failed to discover resources: %v", err)
	}

	excludedGroups := getExcludedGroups(".get-resources-excluded-groups")

	for _, group := range apiResources {
		gv, err := schema.ParseGroupVersion(group.GroupVersion)
		if err != nil {
			continue
		}

		if excludedGroups[gv.Group] {
			continue
		}

		for _, resource := range group.APIResources {
			// Skip subresources like "pods/status"
			if strings.Contains(resource.Name, "/") {
				continue
			}

			// Skip corev1 events
			if gv.Group == "" && gv.Version == "v1" && (resource.Name == "events" || resource.Kind == "Event") {
				continue
			}

			gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: resource.Name}

			if resource.Namespaced && processNamespacedResources {
				if namespaces == nil || (len(namespaces) == 1 && namespaces[0] == "*") {
					// List all namespaces
					list, err := dyn.Resource(gvr).Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
					if err != nil {
						continue
					}
					filterAndOutput(list.Items, gvr, filter)
				} else {
					// List selected namespaces
					for _, ns := range namespaces {
						// log.Printf("Listing GVR %s (group=%s, version=%s) in namespace %s", gvr.Resource, gvr.Group, gvr.Version, ns)
						list, err := dyn.Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
						if err != nil {
							continue
						}
						filterAndOutput(list.Items, gvr, filter)
					}
				}
			} else if !resource.Namespaced && includeCluster {
				list, err := dyn.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					continue
				}
				filterAndOutput(list.Items, gvr, filter)
			}
		}
	}
	log.Println("Done collecting resources.")
}

// Output and filtering
func filterAndOutput(items []unstructured.Unstructured, gvr schema.GroupVersionResource, filter ResourceFilter) {
	csv_writer := csv.NewWriter(os.Stdout)
	defer csv_writer.Flush()
	for _, item := range items {
		created := item.GetCreationTimestamp().Time
		if !filter.Before.IsZero() && !created.Before(filter.Before) {
			continue
		}
		if !filter.After.IsZero() && !created.After(filter.After) {
			continue
		}
		if !filter.Start.IsZero() && (created.Before(filter.Start) || created.After(filter.End)) {
			continue
		}

		out, err := item.MarshalJSON()
		if err != nil {
			log.Printf("Error marshalling %s: %v", item.GetName(), err)
			continue
		}

		if filter.OutputDir != "" {
			dir := filepath.Join(filter.OutputDir, item.GetNamespace(), gvr.Resource)
			_ = os.MkdirAll(dir, 0755)
			var filename string
			if strings.HasSuffix(gvr.Group, "openshift.io") {
				filename = fmt.Sprintf("openshift_%s.yaml", item.GetName())
			} else {
				filename = fmt.Sprintf("%s.yaml", item.GetName())
			}
			file := filepath.Join(dir, filename)
			f, err := os.Create(file)
			if err != nil {
				log.Printf("Failed to create file: %v", err)
				continue
			}
			writeYAML(out, f)
			f.Close()
		} else if filter.ResourceData {
			err = csv_writer.Write([]string{item.GetKind(), gvr.Resource, item.GetAPIVersion(), item.GetNamespace(), item.GetName(), item.GetCreationTimestamp().UTC().Format(time.RFC3339), string(out)})
		} else {
			err = csv_writer.Write([]string{item.GetKind(), gvr.Resource, item.GetAPIVersion(), item.GetNamespace(), item.GetName(), item.GetCreationTimestamp().UTC().Format(time.RFC3339)})
		}
		if err != nil {
			log.Println("Failed to write data to CSV writer", err)
		}
	}
}

func writeYAML(raw []byte, f *os.File) {
	y := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 1024)
	var obj map[string]interface{}
	_ = y.Decode(&obj)

	scheme := runtime.NewScheme()
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)

	contentType := runtime.ContentTypeYAML
	if isJSON(raw) {
		contentType = runtime.ContentTypeJSON
	}

	unstructuredObj := &runtime.Unknown{Raw: raw, ContentType: contentType}
	_ = serializer.Encode(unstructuredObj, f)
}

func isJSON(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}
