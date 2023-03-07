package main

import (
	"context"
	"flag"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // apimachineryはkubernetesのAPIオブジェクト（Schema, typing, encoding, decoding）を扱うためのライブラリ
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir" // client-goはkubernetesクラスターにリクエストを送る際のクライアントライブラリ
	"path/filepath"
)

// Podオブジェクトのリストを取得する
func main() {
	var defaultKubeConfigPath string
	if home := homedir.HomeDir(); home != "" {
		// build kubeconfig path from $HOME dir
		defaultKubeConfigPath = filepath.Join(home, ".kube", "config") // ~/.kube/configをdefaultKubeConfigPathに代入
	}

	// kubeconfigという名前のflagを定義。
	// デフォルトの値はdefaultKubeConfigPath
	// descriptionは"kubeconfig config file"
	// main.goの実行時に-kubeconfigオプションを指定することで、defaultKubeConfigPathの値を上書きできる
	// go run main.go -hでオプションの一覧を確認できる
	kubeconfig := flag.String("kubeconfig", defaultKubeConfigPath, "kubeconfig config file")
	flag.Parse()

	// kubeconfigを使って、kubernetesクラスターにリクエストを送るためのconfig(restclient.Config)を作成
	config, _ := clientcmd.BuildConfigFromFlags("", *kubeconfig)

	// configを使って、kubernetesクラスターにリクエストを送るためのclientsetを作成
	clientset, _ := kubernetes.NewForConfig(config)

	// clientsetを使って、kubernetesクラスターの全namespaceにあるPodオブジェクトのリストを取得
	// Pods("")は、全namespaceを指定。""にはnamespace名を指定することもできる
	pods, _ := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	// 取得したPodオブジェクトのリストを表示
	fmt.Println("INDEX\tNAMESPACE\tNAME")
	for i, pod := range pods.Items {
		fmt.Printf("%d\t%s\t%s\n", i, pod.GetNamespace(), pod.GetName())
	}
}
