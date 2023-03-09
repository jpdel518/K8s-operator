package main

import (
	"context"
	"fmt"
	samplev1alpha1 "github.com/jpdel518/clientgo-foo-controller/pkg/apis/example.com/v1alpha1"
	clientset "github.com/jpdel518/clientgo-foo-controller/pkg/generated/clientset/versioned"
	informers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/informers/externalversions/example.com/v1alpha1"
	listers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/listers/example.com/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

type Controller struct {
	// 標準clientset
	kubeclientset kubernetes.Interface
	// カスタムリソース用のclientset
	sampleClient     clientset.Interface
	deploymentSynced cache.InformerSynced
	deploymentLister appslisters.DeploymentLister
	foosSynced       cache.InformerSynced // Informerの中にあるキャッシュがsyncされているかどうかを判定する関数
	foosLister       listers.FooLister
	workqueue        workqueue.RateLimitingInterface
}

func NewController(
	kubeclientset kubernetes.Interface,
	sampleClient clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	fooInformer informers.FooInformer) *Controller {
	// コントローラーの初期化
	controller := &Controller{
		kubeclientset:    kubeclientset,
		sampleClient:     sampleClient,
		deploymentSynced: deploymentInformer.Informer().HasSynced,
		deploymentLister: deploymentInformer.Lister(),
		foosSynced:       fooInformer.Informer().HasSynced,
		foosLister:       fooInformer.Lister(),
		workqueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "foo"),
	}

	// Informerにイベントハンドラの登録
	fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleAdd,
		// DeleteFunc: controller.handleDelete,
	})

	return controller
}

func (c *Controller) Run(stopCh chan struct{}) error {
	if ok := cache.WaitForCacheSync(stopCh, c.foosSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
	return nil
}

func (c *Controller) runWorker() {
	klog.Info("runWorker is called")
	for c.processNextWorkItem() {

	}

}

func (c *Controller) handleAdd(obj interface{}) {
	klog.Info("handleAdd is called")
	c.enqueueFoo(obj)
}

// func (c *Controller) handleDelete(obj interface{}) {
// 	klog.Info("handleDelete is called")
// 	c.enqueueFoo(obj)
// }

func (c *Controller) enqueueFoo(obj interface{}) {
	var key string
	var err error
	// objの中からNamespaceとキーを取り出す
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		klog.Errorf("failed to get key from cache %s", err.Error())
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) processNextWorkItem() bool {
	// workqueueがシャットダウン状態であれば終了
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// wrap this block in a func to use defer c.workqueue.Done
	err := func(obj interface{}) error {
		// call Done to tell workqueue that the item was finished processing
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		// string型に変換
		if key, ok = obj.(string); !ok {
			// string型に変換できなかった場合には対象のobjをqueueの中からForgetしてスキップする
			c.workqueue.Forget(obj)
			klog.Errorf("expected string in workqueue but got %#v", obj)
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			// RateLimitがOKって言った時にqueueにアイテムを戻す（時間を置いて再度処理する）
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// requeueされないようにqueueの中から対象のobjを削除する
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	// queueがシャットダウンされている場合のみ呼び出し元のloop処理を終了したいのでerrorがあっても返り値はtrue
	// 呼び出し元のcontroller.runworkerを要確認
	if err != nil {
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	// keyはnameとnamespaceからなっているので、splitして切り分ける
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("failed to split key into namespace and name %s", err.Error())
		return err
	}

	// foosListerを使ってFooを取得する
	foo, err := c.foosLister.Foos(ns).Get(name)
	if err != nil {
		klog.Errorf("failed to get foo resource from lister %s", err.Error())
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	deploymentName := foo.Spec.DeploymentName
	if deploymentName == "" {
		klog.Errorf("deploymentName must be specified %s", key)
		return nil
	}
	deployment, err := c.deploymentLister.Deployments(foo.Namespace).Get(deploymentName)
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Create(context.TODO(), newDeployment(foo), metav1.CreateOptions{})
	}

	if err != nil {
		return err
	}

	klog.Infof("deployment %s is valid", deployment.Name)

	return nil
}

func newDeployment(foo *samplev1alpha1.Foo) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "nginx",
		"controller": foo.Name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            foo.Spec.DeploymentName,
			Namespace:       foo.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(foo, samplev1alpha1.SchemeGroupVersion.WithKind("Foo"))},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: foo.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}
