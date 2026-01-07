package k8s

import (
	"github.com/warriorguo/cicd-platform/backend/internal/models"
)

// This file contains Tekton pipeline-specific functionality
// Currently commented out due to missing Tekton dependencies
// Uncomment and add tektoncd/pipeline dependency when ready to use


type BuildPipelineRunRequest struct {
	AppName           string
	GitURL            string
	Branch            string
	CommitSHA         string
	BuildType         string // "dockerfile" or "docker-compose"
	DockerfilePath    string
	ContextPath       string
	ImageRepo         string
	ImageTag          string
	GitSecretRef      string
	RegistrySecretRef string
}

type DeployPipelineRunRequest struct {
	AppName          string
	ImageRepo        string
	ImageTag         string
	TargetNamespace  string
	TargetDeployName string
	Replicas         int
	EnvVars          models.EnvVars
	MaxUnavailable   int
}

/*
func (c *Client) CreateCICDPipelineRun(ctx context.Context, req *PipelineRunRequest) (*tektonv1.PipelineRun, error) {
	pipelineRunName := fmt.Sprintf("%s-%s", req.AppName, strings.ToLower(req.CommitSHA[:7]))
	
	params := []tektonv1.Param{
		{Name: "git-url", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.GitURL}},
		{Name: "git-revision", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.Branch}},
		{Name: "build-type", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.BuildType}},
		{Name: "dockerfile-path", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.DockerfilePath}},
		{Name: "context-path", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.ContextPath}},
		{Name: "image-repo", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.ImageRepo}},
		{Name: "image-tag", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.ImageTag}},
		{Name: "deploy-namespace", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.TargetNamespace}},
		{Name: "deploy-name", Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.TargetDeployName}},
	}

	if req.GitSecretRef != "" {
		params = append(params, tektonv1.Param{
			Name: "git-secret", 
			Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.GitSecretRef},
		})
	}

	if req.RegistrySecretRef != "" {
		params = append(params, tektonv1.Param{
			Name: "registry-secret",
			Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: req.RegistrySecretRef},
		})
	}

	pipelineRun := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineRunName,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "cicd-platform",
				"app.kubernetes.io/component": "pipeline",
				"cicd.platform/app":           req.AppName,
				"cicd.platform/commit":        req.CommitSHA,
			},
		},
		Spec: tektonv1.PipelineRunSpec{
			PipelineRef: &tektonv1.PipelineRef{
				Name: "cicd-build-deploy",
			},
			ServiceAccountName: "cicd-pipeline",
			Params:             params,
			Workspaces: []tektonv1.WorkspaceBinding{
				{
					Name: "source-ws",
					VolumeClaimTemplate: &v1.PersistentVolumeClaim{
						Spec: v1.PersistentVolumeClaimSpec{
							AccessModes: []v1.PersistentVolumeAccessMode{
								v1.ReadWriteOnce,
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	return c.CreatePipelineRun(ctx, pipelineRun)
}
*/

/*
func (c *Client) GetPipelineRunStatus(ctx context.Context, name string) (*models.ReleaseStatus, error) {
	pr, err := c.GetPipelineRun(ctx, name)
	if err != nil {
		return nil, err
	}

	var status models.ReleaseStatus
	
	if pr.Status.CompletionTime != nil {
		// Pipeline completed
		if pr.Status.Conditions != nil {
			for _, cond := range pr.Status.Conditions {
				if cond.Type == "Succeeded" {
					if cond.Status == "True" {
						status = models.StatusSuccess
					} else {
						status = models.StatusFailed
					}
					break
				}
			}
		}
	} else if pr.Status.StartTime != nil {
		// Pipeline started but not completed
		status = models.StatusRunning
	} else {
		// Pipeline not started yet
		status = models.StatusPending
	}

	return &status, nil
}

func (c *Client) GetPipelineRunLogs(ctx context.Context, pipelineRunName string) (map[string]string, error) {
	// Get all TaskRuns for this PipelineRun
	taskRuns, err := c.tektonClient.TektonV1().TaskRuns(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRunName),
	})
	if err != nil {
		return nil, err
	}

	logs := make(map[string]string)
	
	for _, tr := range taskRuns.Items {
		taskName := tr.Labels["tekton.dev/pipelineTask"]
		if taskName == "" {
			taskName = tr.Name
		}

		// Get pods for this TaskRun
		pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", tr.Name),
		})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				containerLogs, err := c.GetPodLogs(ctx, pod.Name, container.Name)
				if err != nil {
					logs[fmt.Sprintf("%s/%s", taskName, container.Name)] = fmt.Sprintf("Error getting logs: %v", err)
				} else {
					logs[fmt.Sprintf("%s/%s", taskName, container.Name)] = containerLogs
				}
			}
		}
	}

	return logs, nil
}
*/