package main

import (
	"context"
	"fmt"
	"os"
	
	"github.com/warriorguo/cicd-platform/backend/internal/db"
	"github.com/warriorguo/cicd-platform/backend/internal/k8s"
	"github.com/warriorguo/cicd-platform/backend/internal/models"
	"github.com/warriorguo/cicd-platform/backend/pkg/config"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.Connect(&cfg.Database)
	if err != nil {
		fmt.Printf("Failed to init database: %v\n", err)
		os.Exit(1)
	}

	// Initialize k8s client
	k8sClient, err := k8s.NewClient(cfg.K8s.InCluster, cfg.K8s.KubeConfig, cfg.K8s.PipelineNS)
	if err != nil {
		fmt.Printf("Failed to init k8s client: %v\n", err)
		os.Exit(1)
	}
	
	ctx := context.Background()
	releaseID := uint(21)
	pipelineRunName := "simpletest3-build-8dfd0b3-1767707991"
	
	fmt.Printf("Manually updating logs for release %d, pipeline run %s\n", releaseID, pipelineRunName)
	
	// Get all TaskRuns
	taskRuns, err := k8sClient.GetTaskRunsForPipelineRun(ctx, pipelineRunName)
	if err != nil {
		fmt.Printf("Failed to get task runs: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Found %d TaskRuns\n", len(taskRuns))
	
	for _, taskRun := range taskRuns {
		fmt.Printf("\nTaskRun: %s\n", taskRun.TaskRunName)
		fmt.Printf("  Stage: %s\n", taskRun.TaskName)
		fmt.Printf("  Status: %s\n", taskRun.Status)
		fmt.Printf("  Pod: %s\n", taskRun.PodName)
		fmt.Printf("  Containers: %v\n", taskRun.Containers)
		
		// Update BuildLog status
		for _, containerName := range taskRun.Containers {
			var buildLog models.BuildLog
			result := database.Where("release_id = ? AND task_run_name = ? AND container_name = ?", 
				releaseID, taskRun.TaskRunName, containerName).First(&buildLog)
			
			if result.Error != nil {
				fmt.Printf("  BuildLog not found for container %s\n", containerName)
				continue
			}
			
			// Map status
			var status models.BuildLogStatus
			switch taskRun.Status {
			case "Succeeded":
				status = models.BuildLogStatusCompleted
			case "Failed":
				status = models.BuildLogStatusFailed
			case "Running":
				status = models.BuildLogStatusRunning
			default:
				status = models.BuildLogStatusPending
			}
			
			buildLog.Status = status
			buildLog.StartedAt = taskRun.StartedAt
			buildLog.FinishedAt = taskRun.FinishedAt
			
			// Try to capture logs
			fmt.Printf("  Capturing logs for container %s...\n", containerName)
			logs, err := k8sClient.GetCompletedPodLogs(ctx, taskRun.PodName, containerName)
			if err != nil {
				buildLog.ErrorMessage = fmt.Sprintf("Failed to retrieve logs: %v", err)
				fmt.Printf("    Error: %v\n", err)
			} else {
				buildLog.LogsContent = logs
				buildLog.ErrorMessage = ""
				fmt.Printf("    Success: captured %d bytes\n", len(logs))
			}
			
			database.Save(&buildLog)
		}
	}
	
	fmt.Println("\nDone!")
}