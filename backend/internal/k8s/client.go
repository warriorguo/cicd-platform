package k8s

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Client struct {
	clientset     *kubernetes.Clientset
	// tektonClient  *tektonclient.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
	namespace     string
}

func NewClient(inCluster bool, kubeconfig, namespace string) (*Client, error) {
	var config *rest.Config
	var err error

	if inCluster {
		config, err = rest.InClusterConfig()
	} else {
		if kubeconfig == "" {
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// tektonClient, err := tektonclient.NewForConfig(config)
	// if err != nil {
	//	return nil, fmt.Errorf("failed to create tekton clientset: %w", err)
	// }

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		// tektonClient:  tektonClient,
		dynamicClient: dynamicClient,
		config:        config,
		namespace:     namespace,
	}, nil
}

// func (c *Client) CreatePipelineRun(ctx context.Context, pipelineRun *tektonv1.PipelineRun) (*tektonv1.PipelineRun, error) {
//	return c.tektonClient.TektonV1().PipelineRuns(c.namespace).Create(ctx, pipelineRun, metav1.CreateOptions{})
// }

// func (c *Client) GetPipelineRun(ctx context.Context, name string) (*tektonv1.PipelineRun, error) {
//	return c.tektonClient.TektonV1().PipelineRuns(c.namespace).Get(ctx, name, metav1.GetOptions{})
// }

// func (c *Client) ListPipelineRuns(ctx context.Context, labelSelector string) (*tektonv1.PipelineRunList, error) {
//	return c.tektonClient.TektonV1().PipelineRuns(c.namespace).List(ctx, metav1.ListOptions{
//		LabelSelector: labelSelector,
//	})
// }

// func (c *Client) WatchPipelineRun(ctx context.Context, name string) (<-chan *tektonv1.PipelineRun, <-chan error) {
//	prChan := make(chan *tektonv1.PipelineRun)
//	errChan := make(chan error)
//
//	go func() {
//		defer close(prChan)
//		defer close(errChan)
//
//		watchlist := c.tektonClient.TektonV1().PipelineRuns(c.namespace)
//		watcher, err := watchlist.Watch(ctx, metav1.ListOptions{
//			FieldSelector: fmt.Sprintf("metadata.name=%s", name),
//		})
//		if err != nil {
//			errChan <- err
//			return
//		}
//		defer watcher.Stop()
//
//		for event := range watcher.ResultChan() {
//			if pr, ok := event.Object.(*tektonv1.PipelineRun); ok {
//				select {
//				case prChan <- pr:
//				case <-ctx.Done():
//					return
//				}
//			}
//		}
//	}()
//
//	return prChan, errChan
// }

func (c *Client) GetPodLogs(ctx context.Context, podName, containerName string) (string, error) {
	return c.GetPodLogsInNamespace(ctx, c.namespace, podName, containerName)
}

// GetPodLogsInNamespace gets pod logs from a specific namespace
func (c *Client) GetPodLogsInNamespace(ctx context.Context, namespace, podName, containerName string) (string, error) {
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	buf := make([]byte, 1024)
	var result string
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			result += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return result, nil
}

func (c *Client) StreamPodLogs(ctx context.Context, podName, containerName string) (<-chan string, <-chan error) {
	logChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)

		// Add retry logic for transient failures
		var logs io.ReadCloser
		var err error
		
		for retries := 0; retries < 3; retries++ {
			req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, &v1.PodLogOptions{
				Container: containerName,
				Follow:    true,
				Timestamps: false,
			})

			logs, err = req.Stream(ctx)
			if err == nil {
				break
			}
			
			// Check if pod/container is not ready yet
			if strings.Contains(err.Error(), "is waiting to start") || 
			   strings.Contains(err.Error(), "container not found") {
				time.Sleep(2 * time.Second)
				continue
			}
			
			errChan <- err
			return
		}
		
		if logs == nil {
			errChan <- fmt.Errorf("failed to stream logs after retries")
			return
		}
		defer logs.Close()

		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case logChan <- scanner.Text():
			}
		}
		
		if err := scanner.Err(); err != nil && err != io.EOF {
			errChan <- err
		}
	}()

	return logChan, errChan
}


// CreateBuildPipelineRun creates a build-only Tekton PipelineRun using dynamic client
func (c *Client) CreateBuildPipelineRun(ctx context.Context, req *BuildPipelineRunRequest) (string, error) {
	// Validate required fields
	if req.CommitSHA == "" {
		return "", fmt.Errorf("commit_sha is required")
	}
	if len(req.CommitSHA) < 7 {
		return "", fmt.Errorf("commit_sha must be at least 7 characters long, got: %s", req.CommitSHA)
	}

	pipelineRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "pipelineruns",
	}

	// Generate unique name with timestamp to allow retries
	timestamp := time.Now().Unix()
	pipelineRunName := fmt.Sprintf("%s-build-%s-%d", req.AppName, req.CommitSHA[:7], timestamp)
	
	// Optional: Clean up old failed PipelineRuns for the same app/commit to prevent accumulation
	go c.cleanupOldFailedPipelineRuns(ctx, req.AppName, req.CommitSHA[:7])

	// Build params array
	params := []map[string]interface{}{
		{"name": "git-url", "value": req.GitURL},
		{"name": "git-revision", "value": req.Branch},
		{"name": "build-type", "value": req.BuildType},
		{"name": "dockerfile-path", "value": req.DockerfilePath},
		{"name": "context-path", "value": req.ContextPath},
		{"name": "image-repo", "value": req.ImageRepo},
		{"name": "image-tag", "value": req.ImageTag},
	}

	if req.GitSecretRef != "" {
		params = append(params, map[string]interface{}{
			"name":  "git-secret",
			"value": req.GitSecretRef,
		})
	}

	if req.RegistrySecretRef != "" {
		params = append(params, map[string]interface{}{
			"name":  "registry-secret",
			"value": req.RegistrySecretRef,
		})
	}

	// Create PipelineRun unstructured object
	pipelineRun := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tekton.dev/v1",
			"kind":       "PipelineRun",
			"metadata": map[string]interface{}{
				"name":      pipelineRunName,
				"namespace": c.namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":      "cicd-platform",
					"app.kubernetes.io/component": "build-pipeline",
					"cicd.platform/app":           req.AppName,
					"cicd.platform/commit":        req.CommitSHA,
					"cicd.platform/type":          "build",
				},
			},
			"spec": map[string]interface{}{
				"pipelineRef": map[string]interface{}{
					"name": "cicd-build",
				},
				"taskRunTemplate": map[string]interface{}{
					"serviceAccountName": "pipeline-runner",
				},
				"params": params,
				"workspaces": []map[string]interface{}{
					{
						"name": "source-ws",
						"volumeClaimTemplate": map[string]interface{}{
							"spec": map[string]interface{}{
								"accessModes": []string{"ReadWriteOnce"},
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"storage": "1Gi",
									},
								},
							},
						},
					},
					{
						"name": "dockerconfig-ws",
						"secret": map[string]interface{}{
							"secretName": "kaniko-docker-config-ssl-manual",
						},
					},
				},
			},
		},
	}

	_, err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create Build PipelineRun: %w", err)
	}

	return pipelineRunName, nil
}

// CreateDeployPipelineRun creates a deploy-only Tekton PipelineRun using dynamic client
func (c *Client) CreateDeployPipelineRun(ctx context.Context, req *DeployPipelineRunRequest) (string, error) {
	pipelineRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "pipelineruns",
	}

	// Generate unique name with timestamp
	timestamp := time.Now().Unix()
	pipelineRunName := fmt.Sprintf("%s-deploy-%s-%d", req.AppName, req.ImageTag, timestamp)

	// Build params array
	params := []map[string]interface{}{
		{"name": "image-repo", "value": req.ImageRepo},
		{"name": "image-tag", "value": req.ImageTag},
		{"name": "deploy-namespace", "value": req.TargetNamespace},
		{"name": "deploy-name", "value": req.TargetDeployName},
		{"name": "replicas", "value": fmt.Sprintf("%d", req.Replicas)},
		{"name": "max-unavailable", "value": fmt.Sprintf("%d%%", req.MaxUnavailable)},
		{"name": "service-port", "value": fmt.Sprintf("%d", req.ServicePort)},
	}

	// Add environment variables as JSON string
	if len(req.EnvVars) > 0 {
		envVarsJSON, err := json.Marshal(req.EnvVars)
		if err != nil {
			return "", fmt.Errorf("failed to marshal env vars: %w", err)
		}
		params = append(params, map[string]interface{}{
			"name":  "env-vars",
			"value": string(envVarsJSON),
		})
	}

	// Add service account if specified
	if req.ServiceAccount != "" {
		params = append(params, map[string]interface{}{
			"name":  "service-account",
			"value": req.ServiceAccount,
		})
	}

	// Create PipelineRun unstructured object
	pipelineRun := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tekton.dev/v1",
			"kind":       "PipelineRun",
			"metadata": map[string]interface{}{
				"name":      pipelineRunName,
				"namespace": c.namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":      "cicd-platform",
					"app.kubernetes.io/component": "deploy-pipeline",
					"cicd.platform/app":           req.AppName,
					"cicd.platform/type":          "deploy",
				},
			},
			"spec": map[string]interface{}{
				"pipelineRef": map[string]interface{}{
					"name": "cicd-deploy",
				},
				"taskRunTemplate": map[string]interface{}{
					"serviceAccountName": "pipeline-runner",
				},
				"params": params,
			},
		},
	}

	_, err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create Deploy PipelineRun: %w", err)
	}

	return pipelineRunName, nil
}

// GetPipelineRunStatus gets the status of a PipelineRun
func (c *Client) GetPipelineRunStatus(ctx context.Context, name string) (string, error) {
	pipelineRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "pipelineruns",
	}

	pr, err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get PipelineRun: %w", err)
	}

	// Extract status
	conditions, found, err := unstructured.NestedSlice(pr.Object, "status", "conditions")
	if err != nil || !found || len(conditions) == 0 {
		return "Pending", nil
	}

	// Get the latest condition
	latestCondition := conditions[len(conditions)-1].(map[string]interface{})

	condType, _, _ := unstructured.NestedString(latestCondition, "type")
	status, _, _ := unstructured.NestedString(latestCondition, "status")
	reason, _, _ := unstructured.NestedString(latestCondition, "reason")

	if condType == "Succeeded" {
		if status == "True" {
			return "Success", nil
		} else if status == "False" {
			return "Failed", nil
		}
	}

	if reason == "Running" || reason == "Started" {
		return "Running", nil
	}

	return "Pending", nil
}

// DeletePipelineRun deletes a PipelineRun by name. Kubernetes GCs the owned pods via ownerReferences.
func (c *Client) DeletePipelineRun(ctx context.Context, name string) error {
	pipelineRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "pipelineruns",
	}
	err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PipelineRun %s: %w", name, err)
	}
	return nil
}

func (c *Client) UpdateDeployment(ctx context.Context, namespace, deploymentName, imageTag string) error {
	deploymentsRes := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	deployment, err := c.dynamicClient.Resource(deploymentsRes).Namespace(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %w", deploymentName, err)
	}

	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	if err != nil || !found {
		return fmt.Errorf("failed to get containers from deployment")
	}

	for i, container := range containers {
		if containerMap, ok := container.(map[string]interface{}); ok {
			containerMap["image"] = imageTag
			containers[i] = containerMap
		}
	}

	if err := unstructured.SetNestedSlice(deployment.Object, containers, "spec", "template", "spec", "containers"); err != nil {
		return fmt.Errorf("failed to set containers: %w", err)
	}

	_, err = c.dynamicClient.Resource(deploymentsRes).Namespace(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

// TaskRunInfo contains information about a TaskRun and its pods
type TaskRunInfo struct {
	TaskRunName string
	TaskName    string
	PodName     string
	Containers  []string
	Status      string
	StartedAt   *time.Time
	FinishedAt  *time.Time
}

// GetTaskRunsForPipelineRun gets all TaskRuns and their pods for a PipelineRun
func (c *Client) GetTaskRunsForPipelineRun(ctx context.Context, pipelineRunName string) ([]TaskRunInfo, error) {
	taskRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "taskruns",
	}

	// List TaskRuns with label selector
	taskRunList, err := c.dynamicClient.Resource(taskRunRes).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRunName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list TaskRuns: %v", err)
	}

	var taskRuns []TaskRunInfo

	for _, item := range taskRunList.Items {
		taskRunName := item.GetName()
		labels := item.GetLabels()
		taskName := labels["tekton.dev/pipelineTask"]
		if taskName == "" {
			taskName = taskRunName
		}

		// Extract status
		var taskRunStatus string
		var startedAt, finishedAt *time.Time
		
		conditions, found, err := unstructured.NestedSlice(item.Object, "status", "conditions")
		if found && err == nil && len(conditions) > 0 {
			latestCondition := conditions[len(conditions)-1].(map[string]interface{})
			condType, _, _ := unstructured.NestedString(latestCondition, "type")
			condStatus, _, _ := unstructured.NestedString(latestCondition, "status")
			reason, _, _ := unstructured.NestedString(latestCondition, "reason")

			if condType == "Succeeded" {
				if condStatus == "True" {
					taskRunStatus = "Succeeded"
				} else if condStatus == "False" {
					taskRunStatus = "Failed"
				} else {
					taskRunStatus = "Running"
				}
			} else if reason == "Running" || reason == "Started" {
				taskRunStatus = "Running"
			} else {
				taskRunStatus = "Pending"
			}
		} else {
			taskRunStatus = "Pending"
		}

		// Extract timing info
		if startTimeStr, exists, _ := unstructured.NestedString(item.Object, "status", "startTime"); exists {
			if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				startedAt = &startTime
			}
		}
		if completeTimeStr, exists, _ := unstructured.NestedString(item.Object, "status", "completionTime"); exists {
			if completeTime, err := time.Parse(time.RFC3339, completeTimeStr); err == nil {
				finishedAt = &completeTime
			}
		}

		// Get pods for this TaskRun
		pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", taskRunName),
		})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			var containers []string
			for _, container := range pod.Spec.Containers {
				// Skip non-step containers
				if strings.HasPrefix(container.Name, "step-") {
					containers = append(containers, container.Name)
				}
			}

			if len(containers) > 0 {
				taskRuns = append(taskRuns, TaskRunInfo{
					TaskRunName: taskRunName,
					TaskName:    taskName,
					PodName:     pod.Name,
					Containers:  containers,
					Status:      taskRunStatus,
					StartedAt:   startedAt,
					FinishedAt:  finishedAt,
				})
			}
		}
	}

	return taskRuns, nil
}

// GetRunningTaskRuns gets only the TaskRuns that are currently running for a PipelineRun
func (c *Client) GetRunningTaskRuns(ctx context.Context, pipelineRunName string) ([]TaskRunInfo, error) {
	taskRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "taskruns",
	}

	// List TaskRuns with label selector
	taskRunList, err := c.dynamicClient.Resource(taskRunRes).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRunName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list TaskRuns: %v", err)
	}

	var runningTaskRuns []TaskRunInfo

	for _, item := range taskRunList.Items {
		// Check TaskRun status first
		conditions, found, err := unstructured.NestedSlice(item.Object, "status", "conditions")
		if !found || err != nil {
			continue
		}

		// Extract status
		var taskRunStatus string
		var startedAt, finishedAt *time.Time

		if len(conditions) > 0 {
			latestCondition := conditions[len(conditions)-1].(map[string]interface{})
			condType, _, _ := unstructured.NestedString(latestCondition, "type")
			status, _, _ := unstructured.NestedString(latestCondition, "status")
			reason, _, _ := unstructured.NestedString(latestCondition, "reason")

			if condType == "Succeeded" {
				if status == "True" {
					taskRunStatus = "Succeeded"
				} else if status == "False" {
					taskRunStatus = "Failed"
				} else {
					taskRunStatus = "Running"
				}
			} else if reason == "Running" || reason == "Started" {
				taskRunStatus = "Running"
			} else {
				taskRunStatus = "Pending"
			}
		} else {
			taskRunStatus = "Pending"
		}

		// Only include running TaskRuns
		if taskRunStatus != "Running" {
			continue
		}

		// Extract timing info
		if startTimeStr, exists, _ := unstructured.NestedString(item.Object, "status", "startTime"); exists {
			if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				startedAt = &startTime
			}
		}
		if completeTimeStr, exists, _ := unstructured.NestedString(item.Object, "status", "completionTime"); exists {
			if completeTime, err := time.Parse(time.RFC3339, completeTimeStr); err == nil {
				finishedAt = &completeTime
			}
		}

		taskRunName := item.GetName()
		labels := item.GetLabels()
		taskName := labels["tekton.dev/pipelineTask"]
		if taskName == "" {
			taskName = taskRunName
		}

		// Get pods for this TaskRun
		pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", taskRunName),
		})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			var containers []string
			for _, container := range pod.Spec.Containers {
				// Skip non-step containers
				if strings.HasPrefix(container.Name, "step-") {
					containers = append(containers, container.Name)
				}
			}

			if len(containers) > 0 {
				runningTaskRuns = append(runningTaskRuns, TaskRunInfo{
					TaskRunName: taskRunName,
					TaskName:    taskName,
					PodName:     pod.Name,
					Containers:  containers,
					Status:      taskRunStatus,
					StartedAt:   startedAt,
					FinishedAt:  finishedAt,
				})
			}
		}
	}

	return runningTaskRuns, nil
}

// GetTaskRunStatus gets the detailed status of a specific TaskRun
func (c *Client) GetTaskRunStatus(ctx context.Context, taskRunName string) (*TaskRunInfo, error) {
	taskRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "taskruns",
	}

	taskRun, err := c.dynamicClient.Resource(taskRunRes).Namespace(c.namespace).Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get TaskRun %s: %v", taskRunName, err)
	}

	// Extract status
	var status string
	var startedAt, finishedAt *time.Time

	conditions, found, err := unstructured.NestedSlice(taskRun.Object, "status", "conditions")
	if found && err == nil && len(conditions) > 0 {
		latestCondition := conditions[len(conditions)-1].(map[string]interface{})
		condType, _, _ := unstructured.NestedString(latestCondition, "type")
		condStatus, _, _ := unstructured.NestedString(latestCondition, "status")
		reason, _, _ := unstructured.NestedString(latestCondition, "reason")

		if condType == "Succeeded" {
			if condStatus == "True" {
				status = "Succeeded"
			} else if condStatus == "False" {
				status = "Failed"
			} else {
				status = "Running"
			}
		} else if reason == "Running" || reason == "Started" {
			status = "Running"
		} else {
			status = "Pending"
		}
	} else {
		status = "Pending"
	}

	// Extract timing info
	if startTimeStr, exists, _ := unstructured.NestedString(taskRun.Object, "status", "startTime"); exists {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startedAt = &startTime
		}
	}
	if completeTimeStr, exists, _ := unstructured.NestedString(taskRun.Object, "status", "completionTime"); exists {
		if completeTime, err := time.Parse(time.RFC3339, completeTimeStr); err == nil {
			finishedAt = &completeTime
		}
	}

	labels := taskRun.GetLabels()
	taskName := labels["tekton.dev/pipelineTask"]
	if taskName == "" {
		taskName = taskRunName
	}

	// Get pods for this TaskRun
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", taskRunName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for TaskRun %s: %v", taskRunName, err)
	}

	var podName string
	var containers []string
	if len(pods.Items) > 0 {
		pod := pods.Items[0] // Take the first pod
		podName = pod.Name
		for _, container := range pod.Spec.Containers {
			// Skip non-step containers
			if strings.HasPrefix(container.Name, "step-") {
				containers = append(containers, container.Name)
			}
		}
	}

	return &TaskRunInfo{
		TaskRunName: taskRunName,
		TaskName:    taskName,
		PodName:     podName,
		Containers:  containers,
		Status:      status,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
	}, nil
}

// GetCompletedPodLogs retrieves full logs for a completed pod with retry logic
func (c *Client) GetCompletedPodLogs(ctx context.Context, podName, containerName string) (string, error) {
	// First check if the pod exists
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", fmt.Errorf("pod %s not found - it may have been cleaned up by Tekton", podName)
		}
		return "", fmt.Errorf("failed to get pod: %w", err)
	}

	// Check container status
	var containerStatus *v1.ContainerStatus
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			containerStatus = &status
			break
		}
	}

	if containerStatus == nil {
		return "", fmt.Errorf("container %s not found in pod %s", containerName, podName)
	}

	// Check if container is still running or waiting
	if containerStatus.State.Waiting != nil {
		return "", fmt.Errorf("container is still waiting: %s", containerStatus.State.Waiting.Reason)
	}

	// Try to get logs with retry
	maxRetries := 3
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		logs, err := c.GetPodLogs(ctx, podName, containerName)
		if err == nil {
			return logs, nil
		}
		
		lastErr = err
		
		// Don't retry if pod/container doesn't exist
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			break
		}
		
		// Wait before retry with exponential backoff
		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}
	
	return "", fmt.Errorf("failed to get logs after %d attempts: %w", maxRetries, lastErr)
}

// cleanupOldFailedPipelineRuns removes old failed PipelineRuns for the same app/commit to prevent resource accumulation
func (c *Client) cleanupOldFailedPipelineRuns(ctx context.Context, appName, shortSHA string) {
	pipelineRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "pipelineruns",
	}
	
	// List PipelineRuns with specific labels
	labelSelector := fmt.Sprintf("cicd.platform/app=%s,cicd.platform/commit=%s*", appName, shortSHA)
	
	list, err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		// Silently fail - cleanup is optional
		return
	}
	
	for _, item := range list.Items {
		// Check if PipelineRun failed
		status, exists, err := unstructured.NestedMap(item.Object, "status")
		if !exists || err != nil {
			continue
		}
		
		conditions, exists, err := unstructured.NestedSlice(status, "conditions")
		if !exists || err != nil {
			continue
		}
		
		for _, conditionInterface := range conditions {
			condition, ok := conditionInterface.(map[string]interface{})
			if !ok {
				continue
			}
			
			conditionType, exists, err := unstructured.NestedString(condition, "type")
			if !exists || err != nil || conditionType != "Succeeded" {
				continue
			}
			
			conditionStatus, exists, err := unstructured.NestedString(condition, "status")
			if !exists || err != nil {
				continue
			}
			
			// Delete failed PipelineRuns (status = "False")
			if conditionStatus == "False" {
				name, exists, err := unstructured.NestedString(item.Object, "metadata", "name")
				if exists && err == nil {
					// Don't delete the current one we just created
					if !strings.Contains(name, fmt.Sprintf("-%d", time.Now().Unix())) {
						c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
					}
				}
			}
		}
	}
}

// CleanupCompletedPipelineRuns deletes completed PipelineRuns older than maxAge.
// It scopes to PipelineRuns created by this platform via the app.kubernetes.io/name=cicd-platform label.
// Returns the number of PipelineRuns deleted.
func (c *Client) CleanupCompletedPipelineRuns(ctx context.Context, maxAge time.Duration) int {
	pipelineRunRes := schema.GroupVersionResource{
		Group:    "tekton.dev",
		Version:  "v1",
		Resource: "pipelineruns",
	}

	list, err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=cicd-platform",
	})
	if err != nil {
		log.Printf("CleanupCompletedPipelineRuns: failed to list PipelineRuns: %v", err)
		return 0
	}

	deleted := 0
	cutoff := time.Now().Add(-maxAge)

	for _, item := range list.Items {
		name := item.GetName()

		// Check if PipelineRun is finished (Succeeded condition with status True or False)
		conditions, found, err := unstructured.NestedSlice(item.Object, "status", "conditions")
		if err != nil || !found || len(conditions) == 0 {
			continue
		}

		finished := false
		for _, condIface := range conditions {
			cond, ok := condIface.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _, _ := unstructured.NestedString(cond, "type")
			condStatus, _, _ := unstructured.NestedString(cond, "status")
			if condType == "Succeeded" && (condStatus == "True" || condStatus == "False") {
				finished = true
				break
			}
		}
		if !finished {
			continue
		}

		// Check completionTime age
		completionTimeStr, exists, _ := unstructured.NestedString(item.Object, "status", "completionTime")
		if !exists || completionTimeStr == "" {
			continue
		}
		completionTime, err := time.Parse(time.RFC3339, completionTimeStr)
		if err != nil {
			continue
		}
		if completionTime.After(cutoff) {
			continue // Too recent, skip
		}

		// Delete the old PipelineRun
		if err := c.dynamicClient.Resource(pipelineRunRes).Namespace(c.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			log.Printf("CleanupCompletedPipelineRuns: failed to delete %s: %v", name, err)
			continue
		}
		log.Printf("CleanupCompletedPipelineRuns: deleted %s (completed %s)", name, completionTime.Format(time.RFC3339))
		deleted++
	}

	return deleted
}

// ScaleDeployment scales a deployment to the specified number of replicas
func (c *Client) ScaleDeployment(ctx context.Context, namespace, deploymentName string, replicas int32) error {
	deploymentsRes := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	// Get current deployment
	deployment, err := c.dynamicClient.Resource(deploymentsRes).Namespace(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %w", deploymentName, err)
	}

	// Update replicas
	if err := unstructured.SetNestedField(deployment.Object, int64(replicas), "spec", "replicas"); err != nil {
		return fmt.Errorf("failed to set replicas: %w", err)
	}

	// Apply the update
	_, err = c.dynamicClient.Resource(deploymentsRes).Namespace(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}

// GetDeploymentReplicas gets the current replica count for a deployment
func (c *Client) GetDeploymentReplicas(ctx context.Context, namespace, deploymentName string) (int32, int32, error) {
	deploymentsRes := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	deployment, err := c.dynamicClient.Resource(deploymentsRes).Namespace(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get deployment %s: %w", deploymentName, err)
	}

	// Get desired replicas from spec
	specReplicas, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")
	if !found || err != nil {
		return 0, 0, fmt.Errorf("failed to get spec replicas: %w", err)
	}

	// Get ready replicas from status
	readyReplicas, _, _ := unstructured.NestedInt64(deployment.Object, "status", "readyReplicas")

	return int32(specReplicas), int32(readyReplicas), nil
}

// GetDeploymentPods gets information about all pods belonging to a deployment
func (c *Client) GetDeploymentPods(ctx context.Context, namespace, deploymentName string) ([]gin.H, error) {
	// Get pods by label selector (app=deploymentName)
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for deployment %s: %w", deploymentName, err)
	}

	var podInfos []gin.H
	for _, pod := range pods.Items {
		var status string
		var ready bool = false

		// Determine pod status
		switch pod.Status.Phase {
		case v1.PodPending:
			status = "Pending"
		case v1.PodRunning:
			status = "Running"
			// Check if all containers are ready
			ready = true
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					ready = false
					break
				}
			}
		case v1.PodSucceeded:
			status = "Succeeded"
		case v1.PodFailed:
			status = "Failed"
		default:
			status = "Unknown"
		}

		// Override status if there are issues with containers
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				status = "Pending"
				ready = false
			} else if containerStatus.State.Terminated != nil {
				if containerStatus.State.Terminated.ExitCode != 0 {
					status = "Failed"
					ready = false
				}
			}
		}

		// Extract ReplicaSet information
		replicaSetName := ""
		if ownerRefs := pod.GetOwnerReferences(); len(ownerRefs) > 0 {
			for _, ref := range ownerRefs {
				if ref.Kind == "ReplicaSet" {
					replicaSetName = ref.Name
					break
				}
			}
		}

		// Extract image information from the first container
		var image string
		var imageTag string
		if len(pod.Spec.Containers) > 0 {
			image = pod.Spec.Containers[0].Image
			// Extract tag from image (format: registry/repo:tag)
			if idx := strings.LastIndex(image, ":"); idx != -1 {
				imageTag = image[idx+1:]
			}
		}

		podInfos = append(podInfos, gin.H{
			"name":         pod.Name,
			"status":       status,
			"ready":        ready,
			"started_at":   pod.Status.StartTime,
			"node":         pod.Spec.NodeName,
			"replica_set":  replicaSetName,
			"image":        image,
			"image_tag":    imageTag,
			"restart_count": func() int {
				count := 0
				for _, containerStatus := range pod.Status.ContainerStatuses {
					count += int(containerStatus.RestartCount)
				}
				return count
			}(),
		})
	}

	return podInfos, nil
}

// TTYHandler handles WebSocket connections for pod TTY
type TTYHandler struct {
	conn   *websocket.Conn
	resize chan remotecommand.TerminalSize
}

// Read implements io.Reader for websocket
func (t *TTYHandler) Read(p []byte) (int, error) {
	_, message, err := t.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	
	// Handle different message types (JSON messages for control)
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err == nil {
		if msgType, exists := msg["type"]; exists && msgType == "resize" {
			if rows, rowsExists := msg["rows"]; rowsExists {
				if cols, colsExists := msg["cols"]; colsExists {
					size := remotecommand.TerminalSize{
						Width:  uint16(cols.(float64)),
						Height: uint16(rows.(float64)),
					}
					select {
					case t.resize <- size:
					default:
					}
				}
			}
			return 0, nil
		}
	}
	
	// For raw text messages, just pass through
	copy(p, message)
	return len(message), nil
}

// Write implements io.Writer for websocket
func (t *TTYHandler) Write(p []byte) (int, error) {
	err := t.conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Next implements TerminalSizeQueue
func (t *TTYHandler) Next() *remotecommand.TerminalSize {
	size := <-t.resize
	return &size
}

// ExecPodTTY executes commands in a pod with TTY support via WebSocket
func (c *Client) ExecPodTTY(ctx context.Context, namespace, podName, containerName string, conn *websocket.Conn) error {
	// Prepare the API request to execute commands in a pod
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Command:   []string{"/bin/sh"},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	// Create TTY handler
	ttyHandler := &TTYHandler{
		conn:   conn,
		resize: make(chan remotecommand.TerminalSize, 1),
	}

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Stream the command
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:             ttyHandler,
		Stdout:            ttyHandler,
		Stderr:            ttyHandler,
		Tty:               true,
		TerminalSizeQueue: ttyHandler,
	})

	if err != nil {
		return fmt.Errorf("failed to stream: %w", err)
	}

	return nil
}

// GetPodDescribe gets detailed pod information matching kubectl describe pod output
func (c *Client) GetPodDescribe(ctx context.Context, namespace, podName string) (map[string]interface{}, error) {
	// Get the pod
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	// Get events for this pod and related resources
	allEvents := []corev1.Event{}
	
	// Get pod events
	podEvents, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err == nil {
		allEvents = append(allEvents, podEvents.Items...)
	}

	// Get related ReplicaSet events if pod is controlled by one
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" {
			rsEvents, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=ReplicaSet", owner.Name),
			})
			if err == nil {
				allEvents = append(allEvents, rsEvents.Items...)
			}

			// Get Deployment events if ReplicaSet is controlled by one
			rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				for _, rsOwner := range rs.OwnerReferences {
					if rsOwner.Kind == "Deployment" {
						deployEvents, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
							FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment", rsOwner.Name),
						})
						if err == nil {
							allEvents = append(allEvents, deployEvents.Items...)
						}
					}
				}
			}
		}
	}

	// Also get recent general events that might be relevant (last 30 minutes)
	thirtyMinutesAgo := time.Now().Add(-30 * time.Minute)
	generalEvents, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, event := range generalEvents.Items {
			// Include events that mention this pod or are recent warnings/errors
			if event.FirstTimestamp.After(thirtyMinutesAgo) {
				if strings.Contains(event.Message, podName) || 
				   (event.Type == "Warning" && event.FirstTimestamp.After(time.Now().Add(-10*time.Minute))) {
					allEvents = append(allEvents, event)
				}
			}
		}
	}

	// Build comprehensive pod description matching kubectl describe output
	describe := map[string]interface{}{
		"name":      pod.Name,
		"namespace": pod.Namespace,
		"priority":  pod.Spec.Priority,
		"node":      pod.Spec.NodeName,
		"labels":    pod.Labels,
		"annotations": pod.Annotations,
		"status": map[string]interface{}{
			"phase":              pod.Status.Phase,
			"ip":                pod.Status.PodIP,
			"ips":               pod.Status.PodIPs,
			"host_ip":           pod.Status.HostIP,
			"nominated_node":    pod.Status.NominatedNodeName,
			"readiness_gates":   pod.Spec.ReadinessGates,
		},
		"controlled_by": []interface{}{},
		"init_containers": []interface{}{},
		"containers":      []interface{}{},
		"conditions":      []interface{}{},
		"volumes":         []interface{}{},
		"qos_class":       pod.Status.QOSClass,
		"node_selectors":  pod.Spec.NodeSelector,
		"tolerations":     pod.Spec.Tolerations,
		"events":          []interface{}{},
	}

	// Add creation timestamp and age
	if !pod.CreationTimestamp.IsZero() {
		describe["start_time"] = pod.CreationTimestamp.Time
		describe["age"] = time.Since(pod.CreationTimestamp.Time).String()
	}

	// Add owner references (controlled by)
	for _, owner := range pod.OwnerReferences {
		controlledBy := map[string]interface{}{
			"api_version": owner.APIVersion,
			"kind":        owner.Kind,
			"name":        owner.Name,
			"uid":         owner.UID,
			"controller":  *owner.Controller,
		}
		describe["controlled_by"] = append(describe["controlled_by"].([]interface{}), controlledBy)
	}

	// Add init containers
	for _, container := range pod.Spec.InitContainers {
		containerInfo := map[string]interface{}{
			"name":    container.Name,
			"image":   container.Image,
			"ports":   container.Ports,
			"env":     container.Env,
			"mounts":  container.VolumeMounts,
		}
		
		// Add resource limits and requests
		if container.Resources.Limits != nil {
			containerInfo["limits"] = container.Resources.Limits
		}
		if container.Resources.Requests != nil {
			containerInfo["requests"] = container.Resources.Requests
		}
		
		describe["init_containers"] = append(describe["init_containers"].([]interface{}), containerInfo)
	}

	// Add containers with detailed information
	for _, container := range pod.Spec.Containers {
		containerInfo := map[string]interface{}{
			"name":            container.Name,
			"image":           container.Image,
			"image_pull_policy": container.ImagePullPolicy,
			"ports":           container.Ports,
			"host_network":    pod.Spec.HostNetwork,
			"host_pid":        pod.Spec.HostPID,
			"host_ipc":        pod.Spec.HostIPC,
		}

		// Add environment variables
		if len(container.Env) > 0 {
			envVars := []interface{}{}
			for _, env := range container.Env {
				envVar := map[string]interface{}{
					"name":  env.Name,
					"value": env.Value,
				}
				if env.ValueFrom != nil {
					envVar["value_from"] = env.ValueFrom
				}
				envVars = append(envVars, envVar)
			}
			containerInfo["environment"] = envVars
		}

		// Add volume mounts
		if len(container.VolumeMounts) > 0 {
			mounts := []interface{}{}
			for _, mount := range container.VolumeMounts {
				mountInfo := map[string]interface{}{
					"name":       mount.Name,
					"mount_path": mount.MountPath,
					"read_only":  mount.ReadOnly,
				}
				if mount.SubPath != "" {
					mountInfo["sub_path"] = mount.SubPath
				}
				mounts = append(mounts, mountInfo)
			}
			containerInfo["mounts"] = mounts
		}

		// Add resource limits and requests
		if container.Resources.Limits != nil {
			containerInfo["limits"] = container.Resources.Limits
		}
		if container.Resources.Requests != nil {
			containerInfo["requests"] = container.Resources.Requests
		}

		// Add liveness and readiness probes
		if container.LivenessProbe != nil {
			containerInfo["liveness_probe"] = container.LivenessProbe
		}
		if container.ReadinessProbe != nil {
			containerInfo["readiness_probe"] = container.ReadinessProbe
		}

		describe["containers"] = append(describe["containers"].([]interface{}), containerInfo)
	}

	// Add container statuses
	containerStatuses := []interface{}{}
	for _, status := range pod.Status.ContainerStatuses {
		statusInfo := map[string]interface{}{
			"name":          status.Name,
			"ready":         status.Ready,
			"restart_count": status.RestartCount,
			"image":         status.Image,
			"image_id":      status.ImageID,
			"container_id":  status.ContainerID,
			"started":       status.Started,
		}
		
		// Add detailed state information
		if status.State.Running != nil {
			statusInfo["state"] = map[string]interface{}{
				"running": map[string]interface{}{
					"started_at": status.State.Running.StartedAt,
				},
			}
		} else if status.State.Waiting != nil {
			statusInfo["state"] = map[string]interface{}{
				"waiting": map[string]interface{}{
					"reason":  status.State.Waiting.Reason,
					"message": status.State.Waiting.Message,
				},
			}
		} else if status.State.Terminated != nil {
			statusInfo["state"] = map[string]interface{}{
				"terminated": map[string]interface{}{
					"exit_code":   status.State.Terminated.ExitCode,
					"reason":      status.State.Terminated.Reason,
					"message":     status.State.Terminated.Message,
					"started_at":  status.State.Terminated.StartedAt,
					"finished_at": status.State.Terminated.FinishedAt,
					"signal":      status.State.Terminated.Signal,
				},
			}
		}

		// Add last state if available
		if status.LastTerminationState.Terminated != nil {
			statusInfo["last_state"] = map[string]interface{}{
				"terminated": map[string]interface{}{
					"exit_code":   status.LastTerminationState.Terminated.ExitCode,
					"reason":      status.LastTerminationState.Terminated.Reason,
					"message":     status.LastTerminationState.Terminated.Message,
					"started_at":  status.LastTerminationState.Terminated.StartedAt,
					"finished_at": status.LastTerminationState.Terminated.FinishedAt,
				},
			}
		}

		containerStatuses = append(containerStatuses, statusInfo)
	}
	describe["container_statuses"] = containerStatuses

	// Add pod conditions
	for _, condition := range pod.Status.Conditions {
		conditionInfo := map[string]interface{}{
			"type":               condition.Type,
			"status":             condition.Status,
			"last_probe_time":    condition.LastProbeTime,
			"last_transition_time": condition.LastTransitionTime,
			"reason":             condition.Reason,
			"message":            condition.Message,
		}
		describe["conditions"] = append(describe["conditions"].([]interface{}), conditionInfo)
	}

	// Add volumes
	for _, volume := range pod.Spec.Volumes {
		volumeInfo := map[string]interface{}{
			"name": volume.Name,
			"type": "",
		}

		// Determine volume type and add specific details
		if volume.EmptyDir != nil {
			volumeInfo["type"] = "EmptyDir"
			volumeInfo["empty_dir"] = volume.EmptyDir
		} else if volume.HostPath != nil {
			volumeInfo["type"] = "HostPath"
			volumeInfo["host_path"] = volume.HostPath
		} else if volume.PersistentVolumeClaim != nil {
			volumeInfo["type"] = "PVC"
			volumeInfo["claim_name"] = volume.PersistentVolumeClaim.ClaimName
		} else if volume.ConfigMap != nil {
			volumeInfo["type"] = "ConfigMap"
			volumeInfo["config_map"] = volume.ConfigMap
		} else if volume.Secret != nil {
			volumeInfo["type"] = "Secret"
			volumeInfo["secret"] = volume.Secret
		} else if volume.DownwardAPI != nil {
			volumeInfo["type"] = "DownwardAPI"
			volumeInfo["downward_api"] = volume.DownwardAPI
		} else if volume.Projected != nil {
			volumeInfo["type"] = "Projected"
			volumeInfo["projected"] = volume.Projected
		}

		describe["volumes"] = append(describe["volumes"].([]interface{}), volumeInfo)
	}

	// Deduplicate and sort events by timestamp
	seenEvents := make(map[string]bool)
	uniqueEvents := []corev1.Event{}
	for _, event := range allEvents {
		// Create a unique key for each event to avoid duplicates
		key := fmt.Sprintf("%s:%s:%s:%s", event.Type, event.Reason, event.InvolvedObject.Name, event.FirstTimestamp.String())
		if !seenEvents[key] {
			seenEvents[key] = true
			uniqueEvents = append(uniqueEvents, event)
		}
	}
	
	// Sort events by timestamp (oldest first)
	for i := 0; i < len(uniqueEvents); i++ {
		for j := i + 1; j < len(uniqueEvents); j++ {
			if uniqueEvents[i].FirstTimestamp.After(uniqueEvents[j].FirstTimestamp.Time) {
				uniqueEvents[i], uniqueEvents[j] = uniqueEvents[j], uniqueEvents[i]
			}
		}
	}

	// Format events for display
	for _, event := range uniqueEvents {
		eventInfo := map[string]interface{}{
			"type":             event.Type,
			"reason":           event.Reason,
			"age":              time.Since(event.FirstTimestamp.Time).String(),
			"from":             event.Source.Component,
			"message":          event.Message,
			"first_timestamp":  event.FirstTimestamp,
			"last_timestamp":   event.LastTimestamp,
			"count":            event.Count,
			"object":           fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
		}
		describe["events"] = append(describe["events"].([]interface{}), eventInfo)
	}

	return describe, nil
}

// DeleteDeployment deletes a Kubernetes deployment
func (c *Client) DeleteDeployment(ctx context.Context, namespace, deploymentName string) error {
	// Delete the deployment using the apps/v1 API
	deploymentsClient := c.clientset.AppsV1().Deployments(namespace)
	
	// Set the deletion policy to delete immediately
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	
	err := deploymentsClient.Delete(ctx, deploymentName, deleteOptions)
	if err != nil {
		return fmt.Errorf("failed to delete deployment %s in namespace %s: %w", deploymentName, namespace, err)
	}
	
	return nil
}

// IngressConfig holds configuration for creating/updating an Ingress
type IngressConfig struct {
	Name          string
	Namespace     string
	AppName       string
	ServiceName   string
	ServicePort   int
	CommitSHA     string
	Host          string // Required for host-based routing
	Path          string // Path to use (defaults to "/")
	HostTemplate  string // Template like "*.local.playquota.com"
	TLSSecretName string // TLS secret name for HTTPS termination
}

// generateHostFromTemplate generates a host from template and app name
// For example: "*.local.playquota.com" + "myapp" -> "myapp.local.playquota.com"
func generateHostFromTemplate(template, appName string) string {
	if template == "" {
		return ""
	}
	// Replace * with app name
	return strings.Replace(template, "*", appName, 1)
}

// CreateOrUpdateIngress creates or updates an Ingress resource using host-based routing
func (c *Client) CreateOrUpdateIngress(ctx context.Context, config *IngressConfig) error {
	ingressClient := c.clientset.NetworkingV1().Ingresses(config.Namespace)
	
	// Define the Ingress name
	ingressName := fmt.Sprintf("ing-%s", config.AppName)
	if config.Name != "" {
		ingressName = config.Name
	}
	
	// Generate host from template if not provided
	host := config.Host
	if host == "" && config.HostTemplate != "" {
		host = generateHostFromTemplate(config.HostTemplate, config.AppName)
	}
	
	// Use default path if not specified
	path := config.Path
	if path == "" {
		path = "/"
	}
	
	// Define annotations (no rewrite-target for host-based routing with / path)
	annotations := map[string]string{
		"cicd.platform/app":       config.AppName,
		"cicd.platform/commit":    config.CommitSHA,
		"cicd.platform/updatedAt": time.Now().Format(time.RFC3339),
	}
	
	// Create Ingress spec with host-based routing
	pathType := networkingv1.PathTypePrefix
	ingressSpec := networkingv1.IngressSpec{
		IngressClassName: &[]string{"nginx"}[0], // Use ingressClassName instead of annotation
		Rules: []networkingv1.IngressRule{
			{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     path,
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: config.ServiceName,
										Port: networkingv1.ServiceBackendPort{
											Number: int32(config.ServicePort),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add TLS if secret name is configured
	if config.TLSSecretName != "" && host != "" {
		ingressSpec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: config.TLSSecretName,
			},
		}
	}
	
	// Check if Ingress already exists
	existingIngress, err := ingressClient.Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check existing ingress: %w", err)
		}
		
		// Create new Ingress
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        ingressName,
				Namespace:   config.Namespace,
				Annotations: annotations,
			},
			Spec: ingressSpec,
		}
		
		_, err = ingressClient.Create(ctx, ingress, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ingress: %w", err)
		}
		return nil
	}
	
	// Update existing Ingress
	// Check ownership
	if existingApp, ok := existingIngress.Annotations["cicd.platform/app"]; ok && existingApp != config.AppName {
		return fmt.Errorf("ingress %s is owned by another app: %s", ingressName, existingApp)
	}
	
	// Update annotations and spec
	existingIngress.Annotations = annotations
	existingIngress.Spec = ingressSpec
	
	_, err = ingressClient.Update(ctx, existingIngress, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ingress: %w", err)
	}
	
	return nil
}

// DeleteIngress deletes an Ingress resource
func (c *Client) DeleteIngress(ctx context.Context, namespace, appName string) error {
	ingressClient := c.clientset.NetworkingV1().Ingresses(namespace)
	ingressName := fmt.Sprintf("ing-%s", appName)
	
	err := ingressClient.Delete(ctx, ingressName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ingress: %w", err)
	}
	
	return nil
}

// GetIngress retrieves an Ingress resource
func (c *Client) GetIngress(ctx context.Context, namespace, appName string) (*networkingv1.Ingress, error) {
	ingressClient := c.clientset.NetworkingV1().Ingresses(namespace)
	ingressName := fmt.Sprintf("ing-%s", appName)
	
	ingress, err := ingressClient.Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ingress: %w", err)
	}
	
	return ingress, nil
}