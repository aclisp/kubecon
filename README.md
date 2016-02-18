# Sigma 控制台

Sigma 控制台目前只能用于查看容器。其他操作需要使用命令行工具 `kubectl`

## 创建

Create and run a particular image, possibly replicated.
Creates a replication controller to manage the created container(s).

### Usage: 

    kubectl run NAME --image=image [--env="key=value"] [--port=port] [--replicas=replicas] [--dry-run=bool] [--overrides=inline-json] [flags]

### Examples:

    # Start a single instance of nginx.
    $ kubectl run nginx --image=nginx
    
    # Start a single instance of hazelcast and let the container expose port 5701 .
    $ kubectl run hazelcast --image=hazelcast --port=5701
    
    # Start a single instance of hazelcast and set environment variables "DNS_DOMAIN=cluster" and "POD_NAMESPACE=default" in the container.
    $ kubectl run hazelcast --image=hazelcast --env="DNS_DOMAIN=local" --env="POD_NAMESPACE=default"
    
    # Start a replicated instance of nginx.
    $ kubectl run nginx --image=nginx --replicas=5
    
    # Dry run. Print the corresponding API objects without creating them.
    $ kubectl run nginx --image=nginx --dry-run
    
    # Start a single instance of nginx, but overload the spec of the replication controller with a partial set of values parsed from JSON.
    $ kubectl run nginx --image=nginx --overrides='{ "apiVersion": "v1", "spec": { ... } }'
    
    # Start a single instance of nginx and keep it in the foreground, don't restart it if it exits.
    $ kubectl run -i -tty nginx --image=nginx --restart=Never
    
    # Start the nginx container using the default command, but use custom arguments (arg1 .. argN) for that command.
    $ kubectl run nginx --image=nginx -- <arg1> <arg2> ... <argN>
    
    # Start the nginx container using a different command and custom arguments
    $ kubectl run nginx --image=nginx --command -- <cmd> <arg1> ... <argN>

### Flags:

        --attach[=false]: If true, wait for the Pod to start running, and then attach to the Pod as if 'kubectl attach ...' were called.  Default false, unless '-i/--interactive' is set, in which case the default is true.
        --command[=false]: If true and extra arguments are present, use them as the 'command' field in the container, rather than the 'args' field which is the default.
        --dry-run[=false]: If true, only print the object that would be sent, without sending it.
        --env=[]: Environment variables to set in the container
        --generator="": The name of the API generator to use.  Default is 'run/v1' if --restart=Always, otherwise the default is 'run-pod/v1'.
        --hostport=-1: The host port mapping for the container port. To demonstrate a single-machine container.
        --image="": The image for the container to run.
    -l, --labels="": Labels to apply to the pod(s).
        --leave-stdin-open[=false]: If the pod is started in interactive mode or with stdin, leave stdin open after the first attach completes. By default, stdin will be closed after the first attach completes.
        --limits="": The resource requirement limits for this container.  For example, 'cpu=200m,memory=512Mi'
        --no-headers[=false]: When using the default output, don't print headers.
    -o, --output="": Output format. One of: json|yaml|wide|name|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=... See golang template [http://golang.org/pkg/text/template/#pkg-overview] and jsonpath template [http://releases.k8s.io/release-1.1/docs/user-guide/jsonpath.md].
        --output-version="": Output the formatted object with the given version (default api-version).
        --overrides="": An inline JSON override for the generated object. If this is non-empty, it is used to override the generated object. Requires that the object supply a valid apiVersion field.
        --port=-1: The port that this container exposes.
    -r, --replicas=1: Number of replicas to create for this container. Default is 1.
        --requests="": The resource requirement requests for this container.  For example, 'cpu=100m,memory=256Mi'
        --restart="Always": The restart policy for this Pod.  Legal values [Always, OnFailure, Never].  If set to 'Always' a replication controller is created for this pod, if set to OnFailure or Never, only the Pod is created and --replicas must be 1.  Default 'Always'
    -a, --show-all[=false]: When printing, show all resources (default hide terminated pods.)
        --sort-by="": If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. 'ObjectMeta.Name'). The field in the API resource specified by this JSONPath expression must be an integer or a string.
    -i, --stdin[=false]: Keep stdin open on the container(s) in the pod, even if nothing is attached.
        --template="": Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
        --tty[=false]: Allocated a TTY for each container in the pod.  Because -t is currently shorthand for --template, -t is not supported for --tty. This shorthand is deprecated and we expect to adopt -t for --tty soon.

## 常用

* [HTTP REST API](https://61.160.36.122/swagger-ui/)
* [实时资源查看](https://61.160.36.122/api/v1/proxy/namespaces/kube-system/services/kubedash)
* [Jenkins](http://61.160.36.122:9100)
* [私有 Image 仓库](http://61.160.36.122:8080/v2/_catalog)
* [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/)
