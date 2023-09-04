## Prerequisites

Before you start building the `pod-restart-notifier` operator, make sure you have the following software dependencies installed on your machine:

1. Install [Go](https://go.dev/dl/)
2. Install [Minikube](https://minikube.sigs.k8s.io/docs/start/)
3. Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
4. Install [Docker](https://www.docker.com/products/docker-desktop/)
5. Install [Helm](https://helm.sh/docs/intro/install/)

In this guide, we walk through the process of creating a Kubernetes operator called `pod-restart-notifier`. The operator uses Custom Resource Definitions (CRDs) to define custom resources and manages their lifecycle. We leverage `kubebuilder` to simplify the development of CRDs and custom controllers. Additionally, we demonstrate the testing process for the operator in a Minikube Kubernetes cluster.
### Step 1: Create a Local Kubernetes Cluster Using Minikube

We set up a local Kubernetes cluster with Minikube to simulate a multi-node environment for development and testing. The command used for setting up the Minikube cluster is detailed below:

```bash
# Minikube Setup Command
minikube start --nodes=3 --driver=docker --memory=2g --cpus=2 --cni=cilium --profile operator
```

- We're creating a 3-node cluster to simulate a multi-node environment.
- We're using Docker as the underlying technology to isolate our cluster nodes.
- We're allocating 2GB of RAM and 2 CPU cores per node, which is a good starting point for most development tasks.
- We're using Cilium for networking to benefit from its advanced security and networking features.
- We're giving this cluster a profile name "operator" for easier identification and management.

This setup provides a good starting point for most development tasks and allows us to verify that our operator works well in a multi-node environment.

After setting up the Minikube cluster, it's important to verify that the nodes and system components are running as expected.

```bash
kubectl get nodes
```

This command will list all the nodes in the cluster. If all nodes are listed as `Ready`, it confirms that they are operational and connected to the cluster.

```
kubectl -n kube-system get pods
```

This command checks the system components running in the `kube-system` namespace. If all the pods are running or completed, it's a good indication that the cluster is set up correctly.

When cluster is up and running with the following command enable ingress.

```bash
minikube addons enable ingress --profile=operator
```
### Step 2: Prepare pods for Testing

We will deploy 2 pods that will automatically exit roughly every 3 minutes and 20 seconds, prompting Kubernetes to restart them.

Create the `restart-deployment.yaml`:

```bash
touch restart-deployment.yaml

vi restart-deployment.yaml
```

Paste the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: restart-deployment
spec:
  replicas: 2
  selector:
    matchLabels:
      app: restart-pod
  template:
    metadata:
      labels:
        app: restart-pod
    spec:
      containers:
      - name: restart-container
        image: busybox
        command: ["/bin/sh"]
        args: ["-c", "sleep 200; exit 0"]
```

Apply the deployment:

```bash
kubectl apply -f restart-deployment.yaml
```

Monitor the Restarts:

You can observe the pods and their restart counts by running:

```bash
kubectl get pods -w
```

- We will deploy 2 pods that will automatically exit prompting Kubernetes to restart them.

Create the `failing-pod-deploy.yaml`:

```bash
touch failing-pod-deploy.yaml

vi failing-pod-deploy.yaml
```

Paste the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: failing-pod
spec:
  replicas: 2
  selector:
    matchLabels:
      app: failing-app
  template:
    metadata:
      labels:
        app: failing-app
    spec:
      containers:
      - name: failing-container
        image: busybox
        command:
        - /bin/sh
        - -c
        - "exit 1"
```

Apply the deployment:

```bash
kubectl apply -f failing-pod-deploy.yaml
```

Monitor the Restarts:

You can observe the pods and their restart counts by running:

```bash
kubectl get pods -w
```

### Step 3: Install and Configure Kubebuilder

Kubebuilder is a framework for building Kubernetes APIs using custom resource definitions (CRDs). It's a toolkit that provides the essential scaffolding to help developers build CRDs and custom controllers in a standard layout. It leverages the runtime libraries provided by the `controller-runtime` project and integrates well with the Go programming language, offering specific helpers and tooling for Go-based operator development.

```bash
os=$(go env GOOS)
arch=$(go env GOARCH)

curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/${os}/${arch}"

chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
```

```bash
mkdir pod-restart-notifier-crd && cd pod-restart-notifier-crd
```

```bash
kubebuilder init --domain vodafone.com --repo github.com/NoNickeD/pod-restart-notifier-operator-crd

kubebuilder create api --group monitoring --version v1 --kind PodNotifRestart

# Create Resource [y/n]
y
# Create Controller [y/n]
y

kubebuilder create webhook --group monitoring --version v1 --kind PodNotifRestart --defaulting --programmatic-validation
```

- `--defaulting`: For defaulting webhooks, which are responsible for filling in default values for fields.
- `--programmatic-validation`: For validating webhooks, which allow you to add custom validation logic to your resource.
- `--conversion`: For conversion webhooks, which enable custom conversion logic for different versions of your custom resources.
### Step 4: Define Your Custom Resource

The `types.go` file is the heart of our CRD. It defines the schema of our custom resources (`PodNotifRestart`). The schema includes configurable settings such as namespaces to monitor, minimum restarts for notification, and various webhook URLs for different notification services like Discord, Teams, and Slack.

Modify the `api/v1/podnotifrestart_types.go`

```go
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodNotifRestartSpec defines the desired state of PodNotifRestart
type PodNotifRestartSpec struct {
	// NamespacesToMonitor specifies which namespaces to monitor.
	// If empty, all namespaces will be monitored.
	NamespacesToMonitor []string `json:"namespacesToMonitor,omitempty"`

	// MinRestarts specifies the minimum number of restarts before sending a notification.
	MinRestarts int32 `json:"minRestarts,omitempty"`

	// DiscordWebhookURL is the webhook URL for Discord notifications.
	DiscordWebhookURL string `json:"discordWebhookURL,omitempty"`

	// TeamsWebhookURL is the webhook URL for Microsoft Teams notifications.
	TeamsWebhookURL string `json:"teamsWebhookURL,omitempty"`

	// SlackWebhookURL is the webhook URL for Slack notifications.
	SlackWebhookURL string `json:"slackWebhookURL,omitempty"`
}

.....

// AddNotificationSent adds 1 to the number of notifications sent so far.
func (p *PodNotifRestart) AddNotificationSent() {
	p.Status.NotificationsSent++
}
```

Modify the `internal/controller/podnotifrestart_controller.go`

This file contains the main logic of your custom controller, which watches for changes to Pod resources in your cluster and performs reconciliation actions accordingly.

```go
package controllers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/NoNickeD/pod-restart-notifier-operator-crd/api/v1"

	corev1 "k8s.io/api/core/v1"
)

// Notifier interface
type Notifier interface {
	Notify(message string) error
}

....

// postMessage function
func postMessage(webhookURL string, payload string) error {
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("failed to send post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	return nil
}
```




Kubebuilder tries to reduce the complexity involved in writing operators and custom controllers for Kubernetes by giving developers the tools to focus on what makes their operators unique, rather than dealing with the repetitive parts of coding CRUD operations and API interactions.

### Step 5: Generate CRD Manifests and Code

With our resource type defined, we can generate the necessary manifests and client code using `make` commands.

```bash
make manifests
```

**Regenerate the client code:**

```bash
make generate
```
### Step 6: Build and Deploy the Operator

You'll need to build a Docker image containing your operator and push it to a container registry like Docker Hub. This makes it available for deployment to any Kubernetes cluster.

```bash
docker build -t <your_dockerhub_username>/<name_of_image>:<tag> .

docker build -t nonickednn/pod-restart-notifier-operator-crd:0.1.5 .
```

Push the Docker image to a container registry, for example Docker Hub:

```bash
# If you're not already logged in:
docker login

# Push the versioned image
docker push <your_dockerhub_username>/<name_of_image>:<tag>

docker push nonickednn/pod-restart-notifier-operator-crd:0.1.5
```

Replace `<your_dockerhub_username>` with your Docker Hub username.

Finally, to install your operator into your cluster, use Helm to lint and install the operator package. Make sure to replace `YOUR_TEAMS_WEBHOOK_URL` with the webhook URL where you want to receive the notifications.

First, lint the chart to ensure no errors:

```bash
helm lint ./pod-restart-notifier-crd
```

The structure of  Helm chart must be the following:

```bash
./pod-restart-notifier-crd
├── Chart.yaml
├── crds
│   └── crdDefinition.yaml
├── templates
│   ├── NOTES.txt
│   ├── clusterrole.yaml
│   ├── clusterrolebinding.yaml
│   ├── deamonset.yaml
│   ├── podnotifrestart.yaml
│   └── serviceaccount.yaml
└── values.yaml
```

- **`Chart.yaml`**: Metadata about the Helm chart, specifying details like the chart version, Kubernetes API version, and description. This provides information about the chart and its purpose.
- **`crds/crdDefinition.yaml`**: Your Custom Resource Definition file. This defines the schema for the custom resource that your controller will be watching. Helm will apply this file before installing the other Kubernetes resources.
- **`templates/`**: This directory contains templated Kubernetes manifests that will be populated by values from `values.yaml` or overridden during the Helm install/upgrade process.
    - **`NOTES.txt`**: Informational notes that will be displayed to the user after installing or upgrading the chart.
    - **`clusterrole.yaml`**: Defines the ClusterRole for the application, specifying permissions it needs across the Kubernetes cluster.
    - **`clusterrolebinding.yaml`**: Binds the ClusterRole to a specific ServiceAccount, granting it the permissions specified in the ClusterRole.
    - **`daemonset.yaml`**: Defines a DaemonSet, ensuring that your application runs on every (or some) nodes in the Kubernetes cluster.
    - **`podnotifrestart.yaml`**: I assume this is a custom resource manifest for the CRD. This would specify the behavior you want when a pod restarts, as per your CRD.
    - **`serviceaccount.yaml`**: Specifies the ServiceAccount under which the application's pods will run. This is usually tied to the ClusterRole via the ClusterRoleBinding.
- **`values.yaml`**: Default configuration values for the chart. These can be overridden at install time.

For the contents of the files, see the relevant file paths.

Then, package and install:

```bash
# Package
helm package ./pod-restart-notifier
```

```bash
Successfully packaged chart and saved it to: ~/pod-restart-notifier-crd/pod-restart-notifier-crd-0.1.5.tgz
```

```bash
# Install
helm install pod-restart-notifier-crd ./pod-restart-notifier-crd-0.1.5.tgz --set customResource.minRestarts=2 --set customResource.namespacesToMonitor=default --set teams.webhookURL="TEAMS_WEBHOOK_URL"
```
### Step 7: Verify Operator Installation

After deploying the operator using Helm, it's essential to confirm that the operator is running as expected.

1. **Check Helm Releases**: Verify that the Helm release for your operator is listed:

```bash
helm list -n <namespace_where_operator_is_deployed>

helm list -n default
```

You should see `pod-restart-notifier` in the list of Helm releases for the specified namespace.

2. **Check Operator Pod Status**: Next, look for the operator pod and make sure it's running:

```bash
kubectl -n <namespace_where_operator_is_deployed> get pods 

kubectl -n default get pods
```
   
The pod for `pod-restart-notifier-operator-crd` should be in a `Running` or `Completed` state.

3. **Check Custom Resources**: Make sure the operator has created or is managing the custom resources successfully:

```bash
kubectl -n <namespace_where_custom_resource_is_deployed> get podnotifrestarts.monitoring.vodafone.com 

kubectl -n default get podnotifrestarts.monitoring.vodafone.com 
```

If the custom resource is listed, it means the operator is successfully watching over it.

By following these steps, you can confirm that the `pod-restart-notifier` operator has been successfully deployed and is operational. This provides you with assurance that you're ready to proceed with using the operator in your environment.

## Clean up

If you want to delete the cluster entirely, including all data and state, you can use the following command.

```bash
minikube delete --profile=operator
```
### Conclusion

Congratulations! You've successfully created a Kubernetes operator called `pod-restart-notifier`. This operator uses Custom Resource Definitions (CRDs) to monitor pod restarts in specified namespaces and notifies via webhooks to Discord, Teams, or Slack. You've learned how to:

- Set up a local Kubernetes cluster using Minikube.
- Define Custom Resource Definitions with Kubebuilder.
- Build and deploy your operator to a Kubernetes cluster.
- Test the operator in real-time scenarios.

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

