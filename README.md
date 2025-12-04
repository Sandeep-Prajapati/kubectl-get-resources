# kubectl get-resources
<p align="center">
  <a href="https://github.com/Sandeep-Prajapati/kubectl-get-resources">
    <img alt="kubectl get-resources logo" src="assets/get-resources-logo.png" width="140" />
  </a>
</p>

# kubectl get-resources

A kubectl plugin that provides current state visibility into application resources. It works at both namespace and cluster scope, enabling you to capture resource details in CSV format or export them as YAML files directly to your local filesystem.


## Installation (TBD)

Can be installed with [krew](https://krew.sigs.k8s.io/) plugin manager:

    kubectl krew install get-resources
    kubectl get-resources --help

## Usage

```
$ kubectl get-resources --help
Get resources from the K8s/OpenShift cluster. Note: all flags are optional.

Flags:
  -after string
    	Only include resources created after this RFC3339 timestamp
  -before string
    	Only include resources created before this RFC3339 timestamp
  -end string
    	End time for filtering resources (use with --start)
  -exclude-cluster-resources
    	Exclude cluster-scoped resources
  -namespace value
    	Namespace(s) to process. Use '*' for all, '' for only cluster resources.
  -output string
    	Directory to save collected resource YAMLs
  -resource-data
      Add resource details in CSV output
  -start string
    	Start time for filtering resources (use with --end)

Examples:
  Get all resources (namespaced + cluster resources)
  kubectl get-resources

  Get only cluster-scoped resources
  kubectl get-resources --namespace=""

  Get only all namespaced resources
  kubectl get-resources --namespace="*" --exclude-cluster-resources=true

  Get specific namespace resources
  kubectl get-resources --namespace=default

  Get multiple namespace resources
  kubectl get-resources --namespace=default --namespace=sample-namespace

  Get all resources created before a given time
  kubectl get-resources --after=2025-08-10T09:39:09Z

  Get all resources created after a given time
  kubectl get-resources --after=2025-08-10T09:39:09Z

  Get all resources between two times
  kubectl get-resources --start=2025-08-10T09:39:09Z --end=2025-08-10T10:30:02Z

  Get 'default' namespace resources after a given time
  kubectl get-resources --namespace=default --after=2025-08-10T09:39:09Z

  Get resource details added in CSV output
  kubectl get-resources --namespace=default --after=2025-08-10T09:39:09Z --resource-data=true

  Save all output YAMLs to a directory
  kubectl get-resources --output=<Your directory name>

  Save 'default' namespace resources in directory 'default_namespace_resources'
  kubectl get-resources --namespace=default --output=default_namespace_resources

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
```

## Examples

(1) Get all `default` namespace resources created after given 'timestamp'

```
$ kubectl get-resources --namespace=default --exclude-cluster-resources=true --after=2024-10-08T17:25:48Z
kind,plural,apiversion,namespace,name,creationtimestamp
Secret,secrets,v1,default,all-icr-io,2025-08-04T16:03:45Z
Secret,secrets,v1,default,builder-dockercfg-qbtkk,2025-08-04T15:57:28Z
Secret,secrets,v1,default,default-dockercfg-z65dh,2025-08-04T15:57:56Z
Secret,secrets,v1,default,deployer-dockercfg-lwlcb,2025-08-04T15:57:28Z
ServiceAccount,serviceaccounts,v1,default,builder,2025-08-04T15:57:28Z
ServiceAccount,serviceaccounts,v1,default,default,2025-08-04T15:57:56Z
ServiceAccount,serviceaccounts,v1,default,deployer,2025-08-04T15:57:28Z
ConfigMap,configmaps,v1,default,kube-root-ca.crt,2025-08-04T15:57:56Z
ConfigMap,configmaps,v1,default,openshift-service-ca.crt,2025-08-04T15:57:56Z
Endpoints,endpoints,v1,default,kubernetes,2025-08-04T16:00:46Z
Endpoints,endpoints,v1,default,openshift-apiserver,2025-08-04T15:57:24Z
Endpoints,endpoints,v1,default,openshift-oauth-apiserver,2025-08-04T15:57:24Z
Service,services,v1,default,kubernetes,2025-08-04T15:56:58Z
Service,services,v1,default,openshift,2025-08-04T16:18:19Z
Service,services,v1,default,openshift-apiserver,2025-08-04T15:57:24Z
Service,services,v1,default,openshift-oauth-apiserver,2025-08-04T15:57:24Z
Role,roles,rbac.authorization.k8s.io/v1,default,prometheus-k8s,2025-08-04T16:22:10Z
RoleBinding,rolebindings,rbac.authorization.k8s.io/v1,default,prometheus-k8s,2025-08-04T16:22:11Z
RoleBinding,rolebindings,rbac.authorization.k8s.io/v1,default,system:deployers,2025-08-04T15:57:28Z
RoleBinding,rolebindings,rbac.authorization.k8s.io/v1,default,system:image-builders,2025-08-04T15:57:28Z
RoleBinding,rolebindings,rbac.authorization.k8s.io/v1,default,system:image-pullers,2025-08-04T15:57:28Z
EndpointSlice,endpointslices,discovery.k8s.io/v1,default,kubernetes,2025-08-04T16:00:46Z
EndpointSlice,endpointslices,discovery.k8s.io/v1,default,openshift-apiserver-kpjsg,2025-08-04T15:57:56Z
EndpointSlice,endpointslices,discovery.k8s.io/v1,default,openshift-oauth-apiserver-wwhhb,2025-08-04T15:57:56Z
Role,roles,authorization.openshift.io/v1,default,prometheus-k8s,2025-08-04T16:22:10Z
RoleBinding,rolebindings,authorization.openshift.io/v1,default,prometheus-k8s,2025-08-04T16:22:11Z
RoleBinding,rolebindings,authorization.openshift.io/v1,default,system:deployers,2025-08-04T15:57:28Z
RoleBinding,rolebindings,authorization.openshift.io/v1,default,system:image-builders,2025-08-04T15:57:28Z
RoleBinding,rolebindings,authorization.openshift.io/v1,default,system:image-pullers,2025-08-04T15:57:28Z
2025/12/04 05:22:53 Done collecting resources.
```

(2) Save all `default` namespace resources to `default_resources` directory

```
$ kubectl get-resources --namespace=default --exclude-cluster-resources=true --output=default_resources
Done collecting resources.

$ ls default_resources
default

$ tree  default_resources
default_resources
└── default
    ├── configmaps
    │   ├── kube-root-ca.crt.yaml
    │   └── openshift-service-ca.crt.yaml
    ├── endpoints
    │   ├── kubernetes.yaml
    │   ├── openshift-apiserver.yaml
    │   └── openshift-oauth-apiserver.yaml
    ├── endpointslices
    │   ├── kubernetes.yaml
    │   ├── openshift-apiserver-kpjsg.yaml
    │   └── openshift-oauth-apiserver-wwhhb.yaml
    ├── rolebindings
    │   ├── openshift_prometheus-k8s.yaml
    │   ├── openshift_system:deployers.yaml
    │   ├── openshift_system:image-builders.yaml
    │   ├── openshift_system:image-pullers.yaml
    │   ├── prometheus-k8s.yaml
    │   ├── system:deployers.yaml
    │   ├── system:image-builders.yaml
    │   └── system:image-pullers.yaml
    ├── roles
    │   ├── openshift_prometheus-k8s.yaml
    │   └── prometheus-k8s.yaml
    ├── secrets
    │   ├── all-icr-io.yaml
    │   ├── builder-dockercfg-qbtkk.yaml
    │   ├── default-dockercfg-z65dh.yaml
    │   └── deployer-dockercfg-lwlcb.yaml
    ├── serviceaccounts
    │   ├── builder.yaml
    │   ├── default.yaml
    │   └── deployer.yaml
    └── services
        ├── kubernetes.yaml
        ├── openshift-apiserver.yaml
        ├── openshift-oauth-apiserver.yaml
        └── openshift.yaml

9 directories, 29 files
```

**Note:** Using below group exclusions
```
$ cat ~/.get-resources-excluded-groups 
events.k8s.io
metrics.k8s.io
image.openshift.io
packages.operators.coreos.com
# authorization.openshift.io
```

## Author

Sandeep-Prajapati 


## License

Apache 2.0. See [LICENSE](./LICENSE).

---

This is not an official Google project.
