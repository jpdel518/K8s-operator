package main

import (
	"fmt"
	clientset "github.com/jpdel518/clientgo-foo-controller/pkg/generated/clientset/versioned"
	informers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/informers/externalversions/example.com/v1alpha1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"time"
)

type Controller struct {
	sampleClient clientset.Interface
	foosSynced   cache.InformerSynced // Informerの中にあるキャッシュがsyncされているかどうかを判定する関数
}

func NewController(sampleClient clientset.Interface, fooInformer informers.FooInformer) *Controller {
	// コントローラーの初期化
	controller := &Controller{
		sampleClient: sampleClient,
		foosSynced:   fooInformer.Informer().HasSynced,
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
}

func (c *Controller) handleAdd(obj interface{}) {
	klog.Info("handleAdd is called")
}

func (c *Controller) handleDelete(obj interface{}) {
	klog.Info("handleDelete is called")
}
