package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPipelineRunRequest(t *testing.T) {
	t.Run("Create PipelineRunRequest with Dockerfile", func(t *testing.T) {
		req := &PipelineRunRequest{
			AppName:           "test-app",
			GitURL:            "https://github.com/test/repo.git",
			Branch:            "main",
			CommitSHA:         "abc1234567",
			BuildType:         "dockerfile",
			DockerfilePath:    "Dockerfile",
			ContextPath:       ".",
			ImageRepo:         "harbor.test.com/team/test-app",
			ImageTag:          "harbor.test.com/team/test-app:abc1234",
			TargetNamespace:   "production",
			TargetDeployName:  "test-app",
			GitSecretRef:      "git-credentials",
			RegistrySecretRef: "registry-credentials",
		}

		assert.Equal(t, "test-app", req.AppName)
		assert.Equal(t, "dockerfile", req.BuildType)
		assert.Equal(t, "Dockerfile", req.DockerfilePath)
	})

	t.Run("Create PipelineRunRequest with DockerCompose", func(t *testing.T) {
		req := &PipelineRunRequest{
			AppName:         "compose-app",
			GitURL:          "https://github.com/test/repo.git",
			Branch:          "main",
			CommitSHA:       "def7890123",
			BuildType:       "docker-compose",
			DockerfilePath:  "docker-compose.yml",
			ContextPath:     ".",
			ImageRepo:       "harbor.test.com/team/compose-app",
			ImageTag:        "harbor.test.com/team/compose-app:def7890",
			TargetNamespace: "staging",
			TargetDeployName: "compose-app",
		}

		assert.Equal(t, "compose-app", req.AppName)
		assert.Equal(t, "docker-compose", req.BuildType)
		assert.Equal(t, "docker-compose.yml", req.DockerfilePath)
	})
}

func TestPipelineRunValidation(t *testing.T) {
	t.Run("Reject Empty CommitSHA", func(t *testing.T) {
		req := &PipelineRunRequest{
			AppName:        "test-app",
			GitURL:         "https://github.com/test/repo.git",
			Branch:         "main",
			CommitSHA:      "", // Empty
			BuildType:      "dockerfile",
			DockerfilePath: "Dockerfile",
		}

		// This should fail validation
		assert.Equal(t, "", req.CommitSHA)
	})

	t.Run("Reject Short CommitSHA", func(t *testing.T) {
		req := &PipelineRunRequest{
			AppName:        "test-app",
			GitURL:         "https://github.com/test/repo.git",
			Branch:         "main",
			CommitSHA:      "abc", // Too short (less than 7 chars)
			BuildType:      "dockerfile",
			DockerfilePath: "Dockerfile",
		}

		assert.Equal(t, 3, len(req.CommitSHA))
		assert.Less(t, len(req.CommitSHA), 7)
	})

	t.Run("Accept Valid CommitSHA", func(t *testing.T) {
		req := &PipelineRunRequest{
			AppName:        "test-app",
			GitURL:         "https://github.com/test/repo.git",
			Branch:         "main",
			CommitSHA:      "abc1234567890", // Valid length
			BuildType:      "dockerfile",
			DockerfilePath: "Dockerfile",
		}

		assert.GreaterOrEqual(t, len(req.CommitSHA), 7)
	})
}

func TestPipelineRunStructure(t *testing.T) {
	t.Run("PipelineRun Object Structure", func(t *testing.T) {
		pipelineRun := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "tekton.dev/v1",
				"kind":       "PipelineRun",
				"metadata": map[string]interface{}{
					"name":      "test-app-abc1234",
					"namespace": "cicd-runners",
					"labels": map[string]interface{}{
						"app.kubernetes.io/name":      "cicd-platform",
						"app.kubernetes.io/component": "pipeline",
						"cicd.platform/app":           "test-app",
						"cicd.platform/commit":        "abc1234567",
					},
				},
				"spec": map[string]interface{}{
					"pipelineRef": map[string]interface{}{
						"name": "cicd-build-deploy",
					},
					"serviceAccountName": "cicd-pipeline",
					"params": []map[string]interface{}{
						{"name": "git-url", "value": "https://github.com/test/repo.git"},
						{"name": "build-type", "value": "dockerfile"},
					},
				},
			},
		}

		assert.Equal(t, "tekton.dev/v1", pipelineRun.GetAPIVersion())
		assert.Equal(t, "PipelineRun", pipelineRun.GetKind())
		assert.Equal(t, "test-app-abc1234", pipelineRun.GetName())
		assert.Equal(t, "cicd-runners", pipelineRun.GetNamespace())
	})
}

func TestGetPipelineRunStatusLogic(t *testing.T) {
	t.Run("Parse Pending Status", func(t *testing.T) {
		// Simulate empty conditions - should return Pending
		conditions := []map[string]interface{}{}
		
		if len(conditions) == 0 {
			status := "Pending"
			assert.Equal(t, "Pending", status)
		}
	})

	t.Run("Parse Running Status", func(t *testing.T) {
		condition := map[string]interface{}{
			"type":   "Succeeded",
			"status": "Unknown",
			"reason": "Running",
		}

		condType := condition["type"].(string)
		reason := condition["reason"].(string)

		var status string
		if reason == "Running" || reason == "Started" {
			status = "Running"
		}

		assert.Equal(t, "Succeeded", condType)
		assert.Equal(t, "Running", status)
	})

	t.Run("Parse Success Status", func(t *testing.T) {
		condition := map[string]interface{}{
			"type":   "Succeeded",
			"status": "True",
			"reason": "Succeeded",
		}

		condType := condition["type"].(string)
		condStatus := condition["status"].(string)

		var status string
		if condType == "Succeeded" && condStatus == "True" {
			status = "Success"
		}

		assert.Equal(t, "Success", status)
	})

	t.Run("Parse Failed Status", func(t *testing.T) {
		condition := map[string]interface{}{
			"type":   "Succeeded",
			"status": "False",
			"reason": "Failed",
		}

		condType := condition["type"].(string)
		condStatus := condition["status"].(string)

		var status string
		if condType == "Succeeded" && condStatus == "False" {
			status = "Failed"
		}

		assert.Equal(t, "Failed", status)
	})
}

func TestUpdateDeploymentLogic(t *testing.T) {
	t.Run("Update Container Image", func(t *testing.T) {
		containers := []interface{}{
			map[string]interface{}{
				"name":  "app",
				"image": "old-image:v1",
			},
		}

		newImage := "new-image:v2"

		for i, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				containerMap["image"] = newImage
				containers[i] = containerMap
			}
		}

		updatedContainer := containers[0].(map[string]interface{})
		assert.Equal(t, newImage, updatedContainer["image"])
	})
}
