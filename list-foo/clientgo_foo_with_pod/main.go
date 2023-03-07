package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"path/filepath"
	"time"
)

var gvr = schema.GroupVersionResource{
	Group:    "example.com",
	Version:  "v1alpha1",
	Resource: "foos",
}

type Foo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	TestString string `json:"testString"`
	TestNum    int    `json:"testNum"`
}

type FooList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Foo `json:"items"`
}

func listFoos(client dynamic.Interface, namespace string) (*FooList, error) {
	// list is *unstructured.UnstructuredList
	// UnstructuredListは、CustomResourceで定義した未定義のkubernetesのAPIオブジェクトのリストを表す構造体
	list, err := client.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// MarshalJSONは、UnstructuredListをJSONに変換するためのメソッド
	data, err := list.MarshalJSON()
	if err != nil {
		return nil, err
	}

	// 自分で定義したFooList構造体にマッピング
	var fooList FooList
	if err := json.Unmarshal(data, &fooList); err != nil {
		return nil, err
	}
	return &fooList, nil
}

func createPod(clientset *kubernetes.Clientset, namespace, name string) error {
	// 作成するPodを定義
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "busybox",
					Image:           "gcr.io/google_containers/echoserver:1.4",
					ImagePullPolicy: "IfNotPresent",
				},
			},
			RestartPolicy: v1.RestartPolicyAlways,
		},
	}
	_, err := clientset.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("failed to create Pod %v\n", err)
		return err
	}
	fmt.Printf("Successfully created a Pod (%s)", name)
	return nil
}

// Fooオブジェクトが作成されたら、同じ名前のPodを作成する
// Loopし続けて、Fooオブジェクトが存在する限りPodを削除してもそれを監視して再作成する
// 簡易Controllerのようなもの
func main() {
	var defaultKubeConfigPath string
	if home := homedir.HomeDir(); home != "" {
		// build kubeconfig path from $HOME dir
		defaultKubeConfigPath = filepath.Join(home, ".kube", "config")
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
	client, _ := dynamic.NewForConfig(config)
	// 標準APIオブジェクトを操作するためのclientsetも作成
	clientset, _ := kubernetes.NewForConfig(config)

	for {
		// Fooオブジェクトの一覧を取得
		foos, _ := listFoos(client, "")

		// Fooオブジェクトを一つずつ処理
		for i, foo := range foos.Items {
			namespace := foo.GetNamespace()
			name := foo.GetName()
			fmt.Printf("%d\t%s\t%s\n", i, namespace, name)
			// Fooオブジェクトの名前と同じ名前のPodが存在するか確認
			_, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// 見つからなかったら対象のFooオブジェクトの名前と同じ名前のPodを作成
					fmt.Println("Pod doesn't exist. Creating new Pod")
					err := createPod(clientset, namespace, name)
					if err != nil {
						fmt.Printf("failed to create pod %v\n", err)
					}
				} else {
					fmt.Printf("failed to get pod %v\n", err)
				}
			} else {
				fmt.Printf("successfully got pod %s\n", name)
			}
		}
		// アクセスが集中しないように1秒待つ
		time.Sleep(1 * time.Second)
	}
}
