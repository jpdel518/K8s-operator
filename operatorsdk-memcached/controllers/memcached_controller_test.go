package controllers

import (
	cachev1alpha1 "github.com/example/operatorsdk-memcached/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

const (
	memcachedApiVersion = "cache.example.com/v1alphav1"
	memcachedKind       = "Memcached"
	memcachedName       = "memcached-sample"
	memcachedNamespace  = "default"
	timeout             = time.Second * 10
	interval            = time.Millisecond * 250
)

var _ = Describe("MemcachedController", func() {
	// 各コンテキストの前にCleanup処理を入れないと前のコンテキストで作成したオブジェクトが残っていて、同じNameSpace, Nameで作れないエラーが発生
	BeforeEach(func() {
		// Clean up Memcached
		memcached := &cachev1alpha1.Memcached{}
		err := k8sClient.Get(
			ctx,
			types.NamespacedName{
				Namespace: memcachedNamespace,
				Name:      memcachedName,
			},
			memcached)
		if err == nil {
			err := k8sClient.Delete(ctx, memcached)
			Expect(err).NotTo(HaveOccurred())
		}
		// Clean up Deployment
		deployment := &appsv1.Deployment{}
		err = k8sClient.Get(
			ctx,
			types.NamespacedName{
				Namespace: memcachedNamespace,
				Name:      memcachedName,
			},
			deployment)
		if err == nil {
			err := k8sClient.Delete(ctx, deployment)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	// Memcachedが作成されたらDeploymentが作成されることをテスト
	Context("When Memcached is created", func() {
		It("Deployment should be created", func() {
			// Memcachedを作成
			memcached := &cachev1alpha1.Memcached{
				TypeMeta: metav1.TypeMeta{
					APIVersion: memcachedApiVersion,
					Kind:       memcachedKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      memcachedName,
					Namespace: memcachedNamespace,
				},
				Spec: cachev1alpha1.MemcachedSpec{
					Size: 3,
				},
			}
			err := k8sClient.Create(ctx, memcached)
			Expect(err).NotTo(HaveOccurred())

			// Memcachedのnameとnamespaceを使ってDeploymentを取得
			deployment := &appsv1.Deployment{}
			// Memcached作成後、deploymentの取得期間が短いとdeploymentが作成されていないためテストが失敗する
			// そのため、gomegaのEventuallyというメソッドを使用
			// Eventuallyでは最終的な結果がこうなってほしいというのを作成することができる（タイムアウトとインターバルを指定して何回も繰り返して実現する）
			Eventually(func() error {
				return k8sClient.Get(
					ctx,
					types.NamespacedName{
						Name:      memcachedName,
						Namespace: memcachedNamespace,
					},
					deployment)
			}, timeout, interval).Should(BeNil())
			// errorがnilであることをテスト
			// Expect(err).NotTo(HaveOccurred())
		})
	})

	// MemcachedのSizeを更新するとDeploymentのreplicasも同じ値に更新されることをテスト
	Context("When Memcached'size is updated", func() {
		It("Deployment's replicas should be updated", func() {
			// MemcachedのSizeを3で作成
			memcached := &cachev1alpha1.Memcached{
				TypeMeta: metav1.TypeMeta{
					APIVersion: memcachedApiVersion,
					Kind:       memcachedKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      memcachedName,
					Namespace: memcachedNamespace,
				},
				Spec: cachev1alpha1.MemcachedSpec{
					Size: 3,
				},
			}
			err := k8sClient.Create(ctx, memcached)
			Expect(err).NotTo(HaveOccurred())

			// Memcachedのnameとnamespaceを使ってDeploymentを取得
			// replicasが3であることをテスト
			deployment := &appsv1.Deployment{}
			Eventually(func() int {
				err := k8sClient.Get(
					ctx,
					types.NamespacedName{
						Name:      memcachedName,
						Namespace: memcachedNamespace,
					},
					deployment)
				if err != nil {
					return 0
				}
				return int(*deployment.Spec.Replicas)
			}, timeout, interval).Should(Equal(3))

			// MemcachedのSizeを2に更新
			memcached.Spec.Size = 2
			err = k8sClient.Update(ctx, memcached)
			Expect(err).NotTo(HaveOccurred())

			// Memcachedのnameとnamespaceを使ってDeploymentを取得
			// replicasが2であることをテスト
			Eventually(func() int {
				err := k8sClient.Get(
					ctx,
					types.NamespacedName{
						Name:      memcachedName,
						Namespace: memcachedNamespace,
					},
					deployment)
				if err != nil {
					return 0
				}
				return int(*deployment.Spec.Replicas)
			}, timeout, interval).Should(Equal(2))
		})
	})

	// It("Should be true", func() {
	// 	Expect(true).To(BeTrue())
	// })
})
