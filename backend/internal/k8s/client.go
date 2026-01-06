package k8s

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	v1 "k8s.io/api/core/v1"
)

type Client struct {
	clientset     *kubernetes.Clientset
	// tektonClient  *tektonclient.Clientset
	dynamicClient dynamic.Interface
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
	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, &v1.PodLogOptions{
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

// CreatePipelineRun creates a Tekton PipelineRun using dynamic client
func (c *Client) CreatePipelineRun(ctx context.Context, req *PipelineRunRequest) (string, error) {
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
	pipelineRunName := fmt.Sprintf("%s-%s-%d", req.AppName, req.CommitSHA[:7], timestamp)
	
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
		{"name": "deploy-namespace", "value": req.TargetNamespace},
		{"name": "deploy-name", "value": req.TargetDeployName},
		{"name": "replicas", "value": fmt.Sprintf("%d", req.Replicas)},
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
					"app.kubernetes.io/component": "pipeline",
					"cicd.platform/app":           req.AppName,
					"cicd.platform/commit":        req.CommitSHA,
				},
			},
			"spec": map[string]interface{}{
				"pipelineRef": map[string]interface{}{
					"name": "cicd-build-deploy",
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
		return "", fmt.Errorf("failed to create PipelineRun: %w", err)
	}

	return pipelineRunName, nil
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

// GetCompletedPodLogs retrieves full logs for a completed pod
func (c *Client) GetCompletedPodLogs(ctx context.Context, podName, containerName string) (string, error) {
	// Use the existing GetPodLogs method but ensure it gets complete logs (not streaming)
	return c.GetPodLogs(ctx, podName, containerName)
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