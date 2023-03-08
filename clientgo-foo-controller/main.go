package main

import (
	"context"
	"flag"
	clientset "github.com/jpdel518/clientgo-foo-controller/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"path/filepath"
)

func main() {
	klog.InitFlags(nil)

	// -kubeconfigでパラメータが指定されたら、その値を使用
	// パラメータが指定されない、かつhomeディレクトリが見つかったら~/.kube/configを使用
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional)")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to kubeconfig file")
	}
	flag.Parse()

	// kubeconfigを使用して*restclient.Configの初期化
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// configを使用してclientsetを取得
	// このclientsetはCode Generatorを使用して作成したFooリソースを扱うことのできるclientset
	exampleClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	// clientsetを使用してFooリソースをリストする(ExampleV1alpha1はグループバージョン)
	foos, err := exampleClient.ExampleV1alpha1().Foos("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		klog.Fatalf("listing foos %s %s", err.Error())
	}
	klog.Infof("length of foos is %d", len(foos.Items))
}
