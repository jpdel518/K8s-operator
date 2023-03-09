package main

import (
	"fmt"
	clientset "github.com/jpdel518/clientgo-foo-controller/pkg/generated/clientset/versioned"
	informers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/informers/externalversions/example.com/v1alpha1"
	listers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/listers/example.com/v1alpha1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

type Controller struct {
	sampleClient clientset.Interface
	foosSynced   cache.InformerSynced // Informerの中にあるキャッシュがsyncされているかどうかを判定する関数
	foosLister   listers.FooLister
	workqueue    workqueue.RateLimitingInterface
}

func NewController(sampleClient clientset.Interface, fooInformer informers.FooInformer) *Controller {
	// コントローラーの初期化
	controller := &Controller{
		sampleClient: sampleClient,
		foosSynced:   fooInformer.Informer().HasSynced,
		foosLister:   fooInformer.Lister(),
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "foo"),
	}

	// Informerにイベントハンドラの登録
	fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleAdd,
		DeleteFunc: controller.handleDelete,
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

func (c *Controller) handleDelete(obj interface{}) {
	klog.Info("handleDelete is called")
	c.enqueueFoo(obj)
}

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
			return err
		}
		// 取得できたFooのSpecをログに出力
		klog.Infof("Got foo %+v", foo.Spec)

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
