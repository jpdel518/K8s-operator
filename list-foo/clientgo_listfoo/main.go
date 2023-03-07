package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // apimachineryはkubernetesのAPIオブジェクト（Schema, typing, encoding, decoding）を扱うためのライブラリ
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic" // client-goはkubernetesクラスターにリクエストを送る際のクライアントライブラリ
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"path/filepath"
)

var gvr = schema.GroupVersionResource{
	Group:    "example.com",
	Version:  "v1alpha1",
	Resource: "foos",
}

type Foo struct {
	// metav1.TypeMeta、metav1.ObjectMetaは、kubernetesのAPIオブジェクトのメタデータを表す構造体
	// 未定義のAPIオブジェクトを構造体に変換するためにプロパティに設定しておく必要がある
	metav1.TypeMeta   `json:",inline"`            // apiVersionとkindを表す
	metav1.ObjectMeta `json:"metadata,omitempty"` // metadataを表す

	TestString string `json:"testString"`
	TestNum    int    `json:"testNum"`
}

type FooList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Foo `json:"items"`
}

func listFoos(clientset dynamic.Interface, namespace string) (*FooList, error) {
	// list is *unstructured.UnstructuredList
	// UnstructuredListは、CustomResourceで定義した未定義のkubernetesのAPIオブジェクトのリストを表す構造体
	list, err := clientset.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
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

// Fooオブジェクトの一覧を取得
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
	clientset, _ := dynamic.NewForConfig(config)

	// clientsetを使って、kubernetesクラスターの全namespaceにあるPodオブジェクトのリストを取得
	// Pods("")は、全namespaceを指定。""にはnamespace名を指定することもできる
	foos, _ := listFoos(clientset, "")
	// 取得したPodオブジェクトのリストを表示
	fmt.Println("INDEX\tNAMESPACE\tNAME")
	for i, foo := range foos.Items {
		fmt.Printf("%d\t%s\t%s\n", i, foo.GetNamespace(), foo.GetName())
	}
}
