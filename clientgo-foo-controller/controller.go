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

const (
	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
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
		AddFunc: controller.enqueueFoo,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueueFoo(newObj)
		},
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

	// 対象のDeploymentがFooにコントロール（FooがオーナーのOwnerReferenceの関係にあるか）されているかどうかを確認
	// 対象のDeploymentがFooにコントロールされてるものでない場合はエラーを返す
	if !metav1.IsControlledBy(deployment, foo) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		klog.Info(msg)
		return fmt.Errorf("%s", msg)
	}

	// FooのreplicasとDeploymentのreplicasを比較して、異なっている場合はkubectlientsetを使用してDeploymentを更新
	if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
		klog.Infof("Foo %s replicas: %d, deployment replicas: %d", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)
		deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Update(context.TODO(), newDeployment(foo), metav1.UpdateOptions{})
	}
	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	err = c.updateFooStatus(foo, deployment)
	if err != nil {
		klog.Errorf("failed to update Foo status for %s", foo.Name)
		return err
	}

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

func (c *Controller) updateFooStatus(foo *samplev1alpha1.Foo, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// fooオブジェクトを DeepCopy()する。DeepCopyはCode Generateで作成されたapis/example.com/v1alpha1/zz_generated_deepcopy.goに定義されている
	fooCopy := foo.DeepCopy()
	// fooオブジェクトのstatusにあるAvailableReplicasの更新
	fooCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Foo resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.sampleClient.ExampleV1alpha1().Foos(foo.Namespace).UpdateStatus(context.TODO(), fooCopy, metav1.UpdateOptions{})
	return err
}
