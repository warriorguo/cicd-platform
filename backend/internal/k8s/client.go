package k8s

import (
	"context"
	"fmt"
	"path/filepath"

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
	logChan := make(chan string)
	errChan := make(chan error)

	go func() {
		defer close(logChan)
		defer close(errChan)

		req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, &v1.PodLogOptions{
			Container: containerName,
			Follow:    true,
		})

		logs, err := req.Stream(ctx)
		if err != nil {
			errChan <- err
			return
		}
		defer logs.Close()

		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := logs.Read(buf)
				if n > 0 {
					logChan <- string(buf[:n])
				}
				if err != nil {
					if err.Error() != "EOF" {
						errChan <- err
					}
					return
				}
			}
		}
	}()

	return logChan, errChan
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