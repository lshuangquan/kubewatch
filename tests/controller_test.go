package controller

import (
	"bufio"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// @Version : 1.0
// @Author  : steven.wong
// @Email   : 'wangxk1991@gamil.com'
// @Time    : 2020/05/20 17:20:13
// Desc     :
func TestRunLog(t *testing.T) {
	var tailLines int64 = 20
	var sinceSeconds int64 = 1589982736
	podLogOpts := corev1.PodLogOptions{
		Follow:       true,
		TailLines:    &tailLines,
		SinceSeconds: &sinceSeconds,
		// Container: "kuboard",
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfigPath := os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			kubeconfigPath = os.Getenv("HOME") + "/.kube/config"
		}
		config, _ = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Print("error in getting access to K8S")
	}

	rs, err := clientset.CoreV1().Pods("kube-system").GetLogs("kuboard-67546bb4bf-d64fq", &podLogOpts).Stream()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer rs.Close()

	// go func() {
	// 	<-t.closed
	// 	rs.Close()
	// }()

	sc := bufio.NewScanner(rs)

	for sc.Scan() {
		fmt.Println(sc.Text())
	}

}
