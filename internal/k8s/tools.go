package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ListPodsResult Pod 列表结果
type ListPodsResult struct {
	Pods []PodInfo `json:"pods"`
}

// PodInfo Pod 信息
type PodInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Ready     string            `json:"ready"`
	Age       string            `json:"age"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// formatK8sError 格式化 Kubernetes API 错误信息
func formatK8sError(err error, operation string) error {
	errStr := err.Error()
	if strings.Contains(errStr, "forcibly closed") || strings.Contains(errStr, "connection reset") {
		return fmt.Errorf("%s failed: connection to Kubernetes API was closed. This may indicate:\n1. Authentication token expired or invalid\n2. Network connectivity issues\n3. API server is rejecting the connection\n\nOriginal error: %w", operation, err)
	}
	if strings.Contains(errStr, "timeout") {
		return fmt.Errorf("%s failed: request to Kubernetes API timed out. Please check network connectivity and API server status.\n\nOriginal error: %w", operation, err)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}

// ListPods 列出指定命名空间中的所有 Pods
// 如果 namespace 为空字符串，则查询所有命名空间
func ListPods(clientset *kubernetes.Clientset, namespace string) (string, error) {
	// 如果 namespace 为空，查询所有命名空间
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", formatK8sError(err, "List pods")
	}

	// 处理空结果
	if pods == nil {
		pods = &corev1.PodList{Items: []corev1.Pod{}}
	}

	var podInfos []PodInfo
	for i, pod := range pods.Items {
		ready := "0/0"
		if len(pod.Status.ContainerStatuses) > 0 {
			readyCount := 0
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					readyCount++
				}
			}
			ready = fmt.Sprintf("%d/%d", readyCount, len(pod.Status.ContainerStatuses))
		}

		age := "Unknown"
		if !pod.CreationTimestamp.IsZero() {
			age = time.Since(pod.CreationTimestamp.Time).Round(time.Second).String()
		}

		podInfo := PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Ready:     ready,
			Age:       age,
		}
		
		// 安全处理 Labels（可能为 nil）
		if pod.Labels != nil {
			podInfo.Labels = pod.Labels
		}
		
		podInfos = append(podInfos, podInfo)
		log.Printf("[ListPods] Processed pod %d: %s/%s", i+1, pod.Namespace, pod.Name)
	}

	result := ListPodsResult{Pods: podInfos}
	
	log.Printf("[ListPods] Marshaling result with %d pods", len(podInfos))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("[ListPods] Error marshaling JSON: %v", err)
		return "", fmt.Errorf("failed to marshal pod list: %w", err)
	}

	log.Printf("[ListPods] Successfully listed pods, result length: %d bytes", len(data))
	return string(data), nil
}

// GetDeploymentStatusResult Deployment 状态结果
type GetDeploymentStatusResult struct {
	Name              string                `json:"name"`
	Namespace         string                 `json:"namespace"`
	Replicas          int32                  `json:"replicas"`
	ReadyReplicas     int32                  `json:"readyReplicas"`
	AvailableReplicas int32                  `json:"availableReplicas"`
	Conditions        []DeploymentCondition `json:"conditions"`
}

// DeploymentCondition Deployment 条件
type DeploymentCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// GetDeploymentStatus 获取 Deployment 状态
func GetDeploymentStatus(clientset *kubernetes.Clientset, namespace, deploymentName string) (string, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return "", formatK8sError(err, "Get deployment status")
	}

	var conditions []DeploymentCondition
	for _, cond := range deployment.Status.Conditions {
		conditions = append(conditions, DeploymentCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Message: cond.Message,
		})
	}

	result := GetDeploymentStatusResult{
		Name:              deployment.Name,
		Namespace:         deployment.Namespace,
		Replicas:          *deployment.Spec.Replicas,
		ReadyReplicas:     deployment.Status.ReadyReplicas,
		AvailableReplicas: deployment.Status.AvailableReplicas,
		Conditions:        conditions,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// RestartPod 重启 Pod（通过删除 Pod 实现，Deployment 会自动重新创建）
func RestartPod(clientset *kubernetes.Clientset, namespace, podName string) (string, error) {
	err := clientset.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil {
		return "", formatK8sError(err, "Restart pod")
	}

	result := map[string]string{
		"message":   fmt.Sprintf("Pod %s in namespace %s has been deleted and will be recreated", podName, namespace),
		"pod":       podName,
		"namespace": namespace,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetPodLogs 获取 Pod 日志
func GetPodLogs(clientset *kubernetes.Clientset, namespace, podName string, tailLines int64) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", formatK8sError(err, "Get pod logs")
	}
	defer podLogs.Close()

	// 读取完整日志
	logs, err := io.ReadAll(podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %w", err)
	}

	result := map[string]interface{}{
		"pod":       podName,
		"namespace": namespace,
		"logs":      string(logs),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ListDeployments 列出指定命名空间中的所有 Deployments
// 如果 namespace 为空字符串，则查询所有命名空间
func ListDeployments(clientset *kubernetes.Clientset, namespace string) (string, error) {
	// 如果 namespace 为空，查询所有命名空间
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	
	deployments, err := clientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", formatK8sError(err, "List deployments")
	}

	var deploymentInfos []map[string]interface{}
	for _, deployment := range deployments.Items {
		deploymentInfos = append(deploymentInfos, map[string]interface{}{
			"name":              deployment.Name,
			"namespace":         deployment.Namespace,
			"replicas":          *deployment.Spec.Replicas,
			"readyReplicas":     deployment.Status.ReadyReplicas,
			"availableReplicas": deployment.Status.AvailableReplicas,
		})
	}

	result := map[string]interface{}{
		"deployments": deploymentInfos,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetServiceInfo 获取 Service 信息
func GetServiceInfo(clientset *kubernetes.Clientset, namespace, serviceName string) (string, error) {
	service, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", formatK8sError(err, "Get service info")
	}

	var ports []map[string]interface{}
	for _, port := range service.Spec.Ports {
		ports = append(ports, map[string]interface{}{
			"name":       port.Name,
			"port":       port.Port,
			"protocol":   port.Protocol,
			"targetPort": port.TargetPort.String(),
		})
	}

	result := map[string]interface{}{
		"name":      service.Name,
		"namespace": service.Namespace,
		"type":      string(service.Spec.Type),
		"clusterIP": service.Spec.ClusterIP,
		"ports":     ports,
		"selector":  service.Spec.Selector,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

