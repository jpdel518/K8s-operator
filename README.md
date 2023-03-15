# Operatorチートシート
* Operatorとは： 制御ループ（制御対象を監視し、理想状態に近づける仕組み）を用いたKubernetesの拡張機能（監視対象を自分で定義）
* Operatorがなぜ必要か： ドメイン知識をコード化して、人間が手動作業しなくてはいけない事柄を減らす事が目的？
* CRD + CustomControllerの組み合わせで構成される
* Controllerとは： コントロールプレーンのコンポーネントの１つ。制御ループを実現。
* Controllerの役割： Kubernetes上に作成されたリソースの現実状態（APIサーバーを通してetcdから取得）を理想状態（APIサーバーを通してetcdから取得したKubernetesオブジェクトのspecフィールド）に近づけるために動く 
* CRD（CustomResourceDefinition）はAPIの拡張を行う。自分が作成するアプリケーション独自の理想状態を宣言的に表す拡張定義
* CRDをデプロイすることで、CustomResourceを取り扱うことができるようになる
* CustomControllerは作成されたCustomResourceのspec定義に従って、理想状態に近づける独自の管理ロジックを提供する
* CustomControllerはコントロールプレーン上ではなく、workerノード上にデプロイされる
* CustomControllerは基本的にDeploymentとしてデプロイされる


## Operatorのインストール（Postgres Operatorの使用）
```shell
# CRDの登録
kubectl create -f https://raw.githubusercontent.com/zalando/postgres-operator/master/manifests/operatorconfiguration.crd.yaml

# デフォルトの設定（ConfigurationParameters）を登録
# https://github.com/zalando/postgres-operator/blob/master/docs/reference/operator_parameters.md
kubectl create -f https://raw.githubusercontent.com/zalando/postgres-operator/master/manifests/postgresql-operator-default-configuration.yaml

# RBAC用のServiceAccount作成
kubectl create -f https://raw.githubusercontent.com/zalando/postgres-operator/master/manifests/operator-service-account-rbac.yaml

# Controllerの作成？
kubectl create -f ./operator/postgres-operator-with-crd.yaml

# カスタムリソースの操作をkubectlの代わりにUIでできるようになるリソース（オプショナル）
kubectl apply -k github.com/zalando/postgres-operator/ui/manifests
```

#### 作成したUIの確認
```shell
kubectl port-forward svc/postgres-operator-ui 8081:80

# localhost:8081
# CustomResourceを実際に作成したりできる
```

#### 登録されたAPI Resourceを確認
```shell
kubectl api-resources | grep zalan

#operatorconfigurations            opconfig     acid.zalan.do/v1                       true         OperatorConfiguration
#postgresqls                       pg           acid.zalan.do/v1                       true         postgresql
```

#### CRDの確認
```shell
# 一覧の取得
kubectl get customresourcedefinition

# 詳細（YAML）を確認
kubectl get customresourcedefinition postgresqls.acid.zalan.do -o yaml
```
***

## kubebuilder
### kubebuilderのプロジェクト作成
#### kubebuilderのインストール
```shell
# https://book.kubebuilder.io/quick-start.html
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
```

###
#### Projectの作成（雛形の作成）
```shell
# domainはCRDに定義するAPIグループのドメイン
# repoはgoモジュールの名前
# M1マックの場合はpluginsでgo/v4-alphaを指定する
kubebuilder init --domain my.domain --repo my.domain/guestbook --plugins=go/v4-alpha
```
#### 作成されるファイルは下記のような感じ
1. Go
    1. go.mod
    1. go.sum
    1. main.go # main関数ではmanagerの初期化、healthチェック、Readyチェック、起動を行う
1. Dockerfile
1. Makefile # コマンドセット
1. config # operatorをkubernetes上にデプロイするためのYAMLファイルが入っている

### API作成
#### APIの作成
```shell
kubebuilder create api --group webapp --version v1 --kind Guestbook
```
#### 作成されるファイルは下記のような感じ
- yaml
   - `config/crd/`:
   - `config/rbac/guestbook_editor_role.yaml`: `ClusterRole`
   - `config/rbac/guestbook_viewer_role.yaml`: `ClusterRole`
   - `config/samples/`: `Guestbook`用のサンプルYAMLファイル
- `api/`: custom resource にタイプを追加するためのディレクトリ
   - `v1/`:
     - `guestbook_types.go`: `Guestbook`の定義
     - `groupversion_info.go`: `Guestbook`のバージョン情報
- `internal/controllers/`:
   - `guestbook_controller.go`: GuestBookのcontroller. controllerのロジックを記載
   - `suite_test.go`: Ginkgo test suite file.
- `cmd`:
   - `main.go`: initで作成されたmain.goが移動？controllerで定義されているmanager登録処理を使用

###
#### api/v1/guestbook_types.goの内容
- 構造体
```go
type Password struct {
	metav1.TypeMeta   `json:",inline"`            // apiVersion, kind（Kubernetesオブジェクトに必須なもの）
	metav1.ObjectMeta `json:"metadata,omitempty"` // name, namespace, labels, annotations（Kubernetesオブジェクトに必須なもの）

	Spec   PasswordSpec   `json:"spec,omitempty"`   // spec（理想状態）
	Status PasswordStatus `json:"status,omitempty"` // status（現在の状態）
}
```
- marker
  - controllerにも実装がある
  - 概要はcontroller側に記載
  - structの中に記載する事で、デフォルト値やvalidationを設定することができる
  - [apiserver/helpers.go#L81-L88](https://github.com/kubernetes/kubernetes/blob/e7a2ce75e5df96ba6ea51d904bf2735397b3e203/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/helpers.go#L81-L88)
```go
// kubernetesのオブジェクトであることを知らせている
// +kubebuilder:object:root=true
// AdditionalPrinterColumns（kubectl getをしたときに任意の項目を表示することができるようになる）の追加
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// AdditionalPrinterColumnsを表示するとAGEが表示されなくなる
// 表示させたい場合は下記
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// structの中の例
type PasswordSpec struct {
  // 最小サイズ8のバリデーション
  // +kubebuilder:validation:Minimum=8
  // デフォルト値を20に設定
  // +kubebuilder:default:=20
  // 必須Spec
  // +kubebuilder:validation:Required
  Length int `json:"length"`
}
```
- init
  - guestbook_types.goをapi groupに登録している
```go
func init() {
	SchemeBuilder.Register(&Password{}, &PasswordList{})
}
```

###
#### internal/controllers/guestbook_controller.go（Controller）の内容
- Reconcilerインターフェースを実装している
- Reconciler
  - 監視対象のオブジェクトが作成、更新、削除された際に呼び出される
  - Queueに登録されたRequestを処理する
  - errorがnilでない場合、またはResult.Requeueがtrueの場合は再度Queueに登録される
```go
type Reconciler interface {
    // Reconcile performs a full reconciliation for the object referred to by the Request.
    // The Controller will requeue the Request to be processed again if an error is non-nil or
    // Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
    Reconcile(context.Context, Request) (Result, error)
}
```
- Request
  - Reconcileの引数として渡される
  - 作成、更新、削除されたオブジェクトの名前とnamespaceが格納されている
```go
type Request struct {
    // NamespacedName is the name and namespace of the object to reconcile.
    types.NamespacedName
}
```
- Result
  - Reconcileの戻り値として渡される
  - RequeueをTrueにすることで、再度Queueに登録される
  - RequeueAfterを指定することで、指定した時間後に再度Queueに登録される
```go
type Result struct {
    // Requeue tells the Controller to requeue the reconcile key.  Defaults to false.
    Requeue bool
    // RequeueAfter if greater than 0, tells the Controller to requeue the reconcile key after the Duration.
    // If the reconcile key is already queued due to a previous reconcile, it is only updated if the new
    // RequeueAfter is sooner than the previously requested RequeueAfter.
    RequeueAfter time.Duration
}
```
- marker
    - 下記のようなコメントはmarkerと呼ばれるもので、kubebuilderがCRDの生成に使用している
    - object generatorってやつが、このmarkerを解析してKubernetesオブジェクトに必要なRuntime Objectの実装を生成する
    - 生成されたRuntime Objectの実装は`api/v1/zz_generated.deepcopy.go`に記載されている
    - ここで設定されたRoleを使用してControllerはReconcile処理内でKubernetesオブジェクトを操作できるようになる
```go
// 例えばsecretのリソースを操作する場合には、専用のRBAC（ServiceAccountのRole）を設定するためのmarkerが必要になる
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create
// 対象リソースのgroupsを確認するには下記のように取得する
// kubectl api-resources | grep secret
// 実行結果は<groups>/<apiVersion>の形で記載されている。apiVersionだけの場合はgroupsは空文字になる

// markerを更新した場合はmake manifestsでServiceAccountのRoleを更新する必要がある
```

###
### Controllerの実装
#### Reconcileの実装
- markerの追加  
internal/controllers/guestbook_controller.go（Controller）の内容に記載
- Passwordオブジェクトの取得
```go
var password secretv1alpha1.Password
if err := r.Get(ctx, req.NamespacedName, &password); err != nil {
    logger.Error(err, "Fetch Password object - failed")
    // client.IgnoreNotFound(err)は、NotFoundエラー（既に削除済みの場合）を無視するためのヘルパー関数
    return ctrl.Result{}, client.IgnoreNotFound(err)
}
```
- Secretオブジェクトの作成
```go
secret := &corev1.Secret{
    ObjectMeta: metav1.ObjectMeta{
        Name:      password.Name,
        Namespace: password.Namespace,
    },
    Data: map[string][]byte{
        "password": []byte("123456789"), // password=123456789
    },
}
err = r.Create(ctx, secret)
```
- Reference関係の作成（ガベージコレクタによる自動削除）
```go
// Password Objectと作成するSecretの間にreferenceを作成
// Password Objectが削除されたらSecretはガベージコレクタに削除される
err := ctrl.SetControllerReference(&password, secret, r.Scheme) // Set owner of this Secret
if err != nil {
    logger.Error(err, "Create Secret object if not exists - failed to set SetControllerReference")
    return ctrl.Result{}, err
}
```

###
### CRDのデプロイ
#### CRDファイル作成（api/v1に記載の内容に従ってCRDが生成、controllerのmarker記述に従ってRBAC用のServiceAccountが更新される）
```shell
make manifests
```
作成されるファイルは下記
- `config/crd/bases/webapp.my.domain_guestbooks.yaml`: `Guestbook`のCRDファイル

###
#### kubernetes上にデプロイ
#### 作成したCRDのapply & Kustomizeのコマンドをbinの下に作成
```shell
make install
```

###
#### CRDの確認
```shell
kubectl api-resources | grep guestbook
```

###
#### CRDの更新
api/v1alpha1/guestbook_types.goの内容を変更した場合には下記を実行する]
例えば、specにfooというプロパティがデフォルトで作成されるが、それを削除
```shell
# specの内容を確認（descriptionには、guestbook_type.go内の対象構造体やそのプロパティの上に記述したコメントが使用される）
kubectl get crd passwords.secret.example.com -o jsonpath='{.spec.versions[].schema.openAPIV3Schema.properties.spec}' | jq

# guestbook_type.goの内容を変更

# CRDの更新
make manifests

# CRDに変更が反映されていることを確認
less config/crd/bases/webapp.my.domain_guestbooks.yaml

# CRDのapply
make install

# specの内容を確認
kubectl get crd passwords.secret.example.com -o jsonpath='{.spec.versions[].schema.openAPIV3Schema.properties.spec}' | jq
```

###
### Controllerのデプロイ
#### Imageの作成（ローカル環境で簡易的に実行したい場合はやらなくてもOK）
#### dockerのbuildをしているだけなので、代わりにdocker buildを実行してもOK
- :latestタグは使用してはいけない
- ローカル環境にあるDockerImageを使用する場合には、imagePullPolicy: IfNotPresent もしくは imagePullPolicy: Neverを指定する必要がある？
```shell
make docker-build IMG=guestbook-controller:test
```
pushできるdockerレジストリを持っている場合は下記  
registry 部分を省略すると docker.io/library が自動補完される -> 絶対失敗する  
docker上（ https://hub.docker.com/repository/ ） でpublicレジストリを作るのが一番簡単そう  
```shell
make docker-build docker-push IMG=<some-registry>/<project-name>:tag
```

###
#### kindクラスターにdockerイメージをロード
```shell
kind load docker-image guestbook-controller:test
```

###
#### kubernetes上にデプロイ
- Namespace, CR, RBAC関係のServiceAccount, controllerのDeploymentが作成される
- controllerがdeploymentとして立ち上がる
```shell
make deploy IMG=guestbook-controller:test
```

###
#### 起動しているDeployment(controller)の確認
```shell
kubectl get po -n guestbook-system
kubectl logs -n guestbook-system <上で取得したPod名>
```

<details>
<summary>controllerの実行（ローカル環境で簡易的に実行する方法</summary>

- DockerImageを作成せずにGoのコードを直接実行する
- 内部的には`go run main.go`を実行している  
```shell
make run
````
running状態が維持し続ける
</details>

###
### CR作成
#### CRの作成（Imageを作らずにgoを簡易実行している場合は別のターミナルで）
```shell
kubectl apply -f config/samples/webapp_v1_guestbook.yaml
```
internal/controllers/guestbook_controller.goのReconcileで処理が行われる  
削除した場合も同様にReconcileが実行される

###
### Cleanup
#### CRDの削除（make undeployする場合には一緒に削除されるので実行不要）
```shell
make uninstall
```

###
#### デプロイしたものを削除
```shell
make undeploy
```
###
### AdmissionWebhook
- AdmissionRequestというのを受け取るHTTPのコールバック
- APIサーバーにリクエストがきたときにオブジェクトをetcdに保存する前にvalidationしたり、デフォルトの値を設定したりする役割
- リクエストが認証認可されていることが確認された後に実行される
- MutatingとValidatingの２つのフェーズがある
- 処理の流れは下記
  1. rulesとclientConfigを作ってAPIサーバーに登録しておく
  1. clientがAPIサーバーにリクエストを投げる（create pod, create deploymentとか）
  1. MutatingAdmissionWebhookにAdmissionReviewというリクエストを送る
  1. 作成のリクエストが既存オブジェクトに対して変更を加える必要があるのかを確認
  1. AdmissionReviewを返却。AdmissionReviewでは↑の確認で変更が必要であれば変更されたものを返す + allowed: true/falseで返却（falseで返すとAPIリクエストがエラーになる。etcdに反映されずにエラーでかえる）
  1. ValidatingAdmissionWebhookにAdmissionReviewというリクエストを送る
  1. Validationチェックを行なって、OKであればAdmissionReviewのallowedをtrueで返却。ダメならfalse。
  1. MutatingとValidatingがOKであればetcdに保存される
  1. レスポンスがclientに返る
-
- KubeBuilderでAdmissionWebhookを実現する方法は下記
  1. kubebuilderコマンドでwebhookを作成（下記を実行してくれる）
     - webhookサーバーをmanagerに登録
     - webhookのためのhandlerを作成
     - handlerをwebhookサーバーに登録
  2. 作成されたwebhookの中にロジックを記載

#### ValidatingAdmissionWebhookの作成
```shell
kubebuilder create webhook --group secret --version v1alpha1 --kind Password --programmatic-validation
```
変更されるファイルは下記
- cmd/main.goに下記が追加  
 webhookサーバーをmanagerに登録
```go
if err = (&secretv1alpha1.Password{}).SetupWebhookWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create webhook", "webhook", "Password")
    os.Exit(1)
}
```
- api/webhook_suite_test.go
- api/password_webhook.go
```go
下記メソッドの中にロジックを実装する
ValidateCreate
ValidateUpdate
ValidateDelete
```
- config/certmanager/  
webhookはTLSで通信する必要があるのでcertificate用のYAMLファイルが用意されている
- config/default/manager_webhook_patch.yaml  
webhookサーバー用のポートをエクスポートしている
- config/default/webhookcainjection_patch.yaml  
webhookの定義ファイル

#### api/password_webhook.goにロジックを実装
#### CertManagerをインストール
```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
```
#### [WEBHOOK]と[CERTMANAGER]のコメントをアンコメント
下記ファイルの[WEBHOOK]と[CERTMANAGER]のセクションをアンコメントする
- config/default/kustomization.yaml
- config/crd/kustomization.yaml

#### mutatingをコメントアウト
今回はvalidatingだけ実行するので、下記ファイルのmutating(MutatingWebhookConfigurationに関するセクション)をコメントアウトしておく
- config/webhook/kustomizeconfig.yaml
- config/default/webhookcainjection_patch.yaml

#### 実行
```shell
make install
```
CertManagerを使っているので簡易実行はできない
```shell
IMG=password-operator:webhook
make docker-build IMG=$IMG
kind load docker-image $IMG
make deploy IMG=$IMG
```
#### 確認
Validationに引っかかるCRを作成してapplyすると動作が確認できる
#### cleanup
```shell
make undeploy
kubectl delete -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
```

## OperatorSDK
- OperatorSDKはKubebuilderをwrapしたもの
- OLM（OperatorLifecycleManager）を使ったデプロイはOperatorSDK特有のもの
###
#### install
```shell
brew install operator-sdk
```
version確認
```shell
operator-sdk version
```
###
#### プロジェクト作成
```shell
operator-sdk init --domain example.com --repo github.com/example/operatorsdk-memcached
```
作成されるフォルダ構成はkubebuilderのものとほぼ同じ  
main.goのmanager作成処理ではオプションを指定することができる。 [manager#Options](https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/manager#Options)

###
#### APIの作成
```shell
operator-sdk create api --group cache --version v1alpha1 --kind Memcached --resource --controller
```

###
#### CDRファイルの生成
config/crd/bases/cache.example.com_memcacheds.yamlが作成される
```shell
make manifests
```

###
#### api/v1plpha1/memcached_types.goを変更したら
```shell
make fmt generate manifests
```
- `fmt`: format go codes
- `generate`: go types -> zz_generated.deepcopy.go
- `manifests`: go types & marker -> yaml (crd, rbac...)

###
#### controllers/memcached_controller.goを変更したら
CRDのインストールとControllerの簡易実行
```shell
make install run
```

### ControllerのTEST
#### Tools
1. [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest): コントローラーはAPIサーバーに依存しているので、テストするにはAPIサーバーが必要になる。テストで最低限必要な`etcd`と`kube-apiserver`を提供するライブラリ。
1. [Ginkgo](https://pkg.go.dev/github.com/onsi/ginkgo): ビヘイビア駆動開発(BDD) testing framework for Golang.
1. [Gomega](https://pkg.go.dev/github.com/onsi/gomega): GinkgoテストフレームワークのMacherライブラリ。

#### 準備
kubebuilderやoperator-sdkでプロジェクトを作成するとControllerと同じ階層にsuite_test.goというファイルができている。  
suite_test.goの中身
- TestAPIs： メインでテストを実行する関数
- BeforeSuite： テストを実行する前に準備処理を実行する関数
- AfterSuite： テストが完了した後にCleanUpする処理を実行する関数  

BeforeSuiteでコントローラーを動かす処理の追加
```go
// Create context with cancel.
ctx, cancel = context.WithCancel(context.TODO())

// Register the schema to manager.
// cfgはclientがどこのapiサーバーに対してアクセスするかという情報が含まれている
k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
Scheme: scheme.Scheme,
})

// Initialize `MemcachedReconciler` with the manager client schema.
// コントローラーの初期化 + Managerへの登録
err = (&MemcachedReconciler{
Client: k8sManager.GetClient(),
Scheme: k8sManager.GetScheme(),
}).SetupWithManager(k8sManager)

// Start the with a goroutine.
// manager.startでコントローラーを実行
go func() {
defer GinkgoRecover()
err = k8sManager.Start(ctx)
Expect(err).ToNot(HaveOccurred(), "failed to run manager")
}()
```
AfterSuiteでcancelを実行する
```go
var _ = AfterSuite(func() {
    cancel()
    By("tearing down the test environment")
    err := testEnv.Stop()
    Expect(err).NotTo(HaveOccurred())
```

#### テスト処理
- テスト用のgoファイルを作成memcached_controller_test.go（controllerと同じ階層にファイル作成）
- テストコードを記述  

ginkgoとgomegaをimport  
Describeにテストコードを記述する
```go
package controllers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MemcachedController", func() {
	It("Should be true", func() {
		Expect(true).To(BeTrue())
	})
})
```

#### テストの実行
```go
make test
```

##
## Built-in Controller（clientGoの使用）
[参考] <https://github.com/kubernetes/sample-controller>

###
#### types作成
```shell
mkdir -p pkg/apis/example.com/v1alpha1
```
- 作成したディレクトリに`doc.go`を作成  
- 同じ階層に`types.go`を作成  
Fooリソースの定義を作成  
```go
import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Foo is a specification for a Foo resource
type Foo struct {
metav1.TypeMeta   `json:",inline"`  // apiVersion, kind..etc
metav1.ObjectMeta `json:"metadata,omitempty"`  // name, namespace..etc

    Spec   FooSpec   `json:"spec"`
    Status FooStatus `json:"status"`
}

// FooSpec is the spec for a Foo resource
type FooSpec struct {
    DeploymentName string `json:"deploymentName"`
    Replicas       *int32 `json:"replicas"`
}

// FooStatus is the status for a Foo resource
type FooStatus struct {
    AvailableReplicas int32 `json:"availableReplicas"`
}

// FooList is a list of Foo resources
type FooList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata"`

    Items []Foo `json:"items"`
}
```
- 同じ階層に`register.go`を作成  
作成したFooとFooListのリソースタイプを登録  
Scheme.AddKnownTypesではruntimeオブジェクトのインタフェースを実装する必要があるが、まだFoo, FooListは実装していないのでエラーがでている状態になる
```go
import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

var (
    // SchemeBuilder initializes a scheme builder
    SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
    // AddToScheme is a global function that registers this API group & version to a scheme 
	// Code Generatorから生成されたClientSetのregister.goのinitで使用される
    AddToScheme = SchemeBuilder.AddToScheme
)

// SchemeGroupVersion is group version used to register these objects.
var SchemeGroupVersion = schema.GroupVersion{
    Group:   "example.com",
    Version: "v1alpha1",
}

func Resource(resource string) schema.GroupResource {
    return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(SchemeGroupVersion,
        &Foo{},
        &FooList{},
    )
    metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
    return nil
}
```

###
#### Code Generator
Kubernetes-style API typesの実装を自動で生成してくれる  
作成される実装は下記
1. Deepcopy： カスタムリソースを作成した際に実装しておかないといけないruntime.Objectインターフェース（今`register.go`でエラーになってるやつ）
1. Clientset： KubernetesのAPIリソースにアクセスするためのClient（今回は作成したカスタムリソースにアクセスできるようなClientを作成する必要がある。built-inのClientsetにはカスタムリソースへアクセスする能力はないので）
1. Informer： 対象となるリソースの変更を検知して、Reconcilerに伝える
1. Lister： ローカルのインメモリキャッシュからカスタムリソースをリストする

###
#### Code Generatorの準備
- 環境変数の設定
```shell
codeGeneratorDir=~/repos/kubernetes/code-generator
```
- クローン
```shell
git clone https://github.com/kubernetes/code-generator.git $codeGeneratorDir
```
下記でcodeGeneratorの使い方を表示することができる
```shell
"${codeGeneratorDir}"/generate-groups.sh
```
- マーカーの追加  
doc.goへマーカーを追加
```go
+// +k8s:deepcopy-gen=package
+// +groupName=example.com

 package v1alpha1
```
1. `// +k8s:deepcopy-gen=package`： パッケージ全体に対してDeepCopyを生成
1. `// +groupName=example.com`： Clientsetを生成する際にグループ名として使用される（デフォルトはパッケージ名になる）

types.goへマーカーを追加  
FooとFooListへマーカーを追加する
```go
+// +genclient
+// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

 // Foo is a specification for a Foo resource
 type Foo struct {
 ...
```
```go
+// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

 // FooList is a list of Foo resources
 type FooList struct {
```
1. `// +genclient`： Clientの動詞関数（create, update, delete, get, list, patch, watch）を生成する。FooListはFooの集まりであり、createなどの関数はないからマーカーを追加しない
1. `// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object`： runtime.Objectを実装するようにDeepCopyを生成する

###
#### Codeの生成
- コマンド概要  
"${codeGeneratorDir}"/generate-groups.sh [generators] [output-package] [apis-package] [groups-versions]
1. `[generators]`： (deepcopy,defaulter,client,lister,informer) or "all"
1. ` [output-package]`： 結果出力先Package名
1. `[apis-package]`： apiまでのPackage
1. `[groups-versions]`： apis-package配下に作成したバージョン
1. `--trim-path-prefix $module`： 指定しないとコマンドを実行したディレクトリ配下にgithub.com~というモジュール名のディレクトリが作成されてしまう
- コマンド実行
```shell
module=github.com/jpdel518/clientgo-foo-controller; "${codeGeneratorDir}"/generate-groups.sh all ${module}/pkg/generated ${module}/pkg/apis example.com:v1alpha1 --go-header-file "${codeGeneratorDir}"/hack/boilerplate.go.txt --trim-path-prefix $module --output-base ./
```
コマンドの中では下記が実行されている
1. Set gobin
```shell
GOBIN="$(go env GOBIN)"
gobin="${GOBIN:-$(go env GOPATH)/bin}"
```
2. deepcopy-gen:
```shell
${gobin}/deepcopy-gen --input-dirs github.com/nakamasato/sample-controller/pkg/apis/example.com/v1alpha1 -O zz_generated.deepcopy --go-header-file /Users/m.naka/repos/kubernetes/code-generator/hack/boilerplate.go.txt --trim-path-prefix github.com/nakamasato/sample-controller
```
3. client-gen:
```shell
${gobin}/client-gen --clientset-name versioned --input-base '' --input github.com/nakamasato/sample-controller/pkg/apis/example.com/v1alpha1 --output-package github.com/nakamasato/sample-controller/pkg/generated/clientset --go-header-file /Users/m.naka/repos/kubernetes/code-generator/hack/boilerplate.go.txt --trim-path-prefix github.com/nakamasato/sample-controller
```
4. lister-gen:
```shell
${gobin}/lister-gen --input-dirs github.com/nakamasato/sample-controller/pkg/apis/example.com/v1alpha1 --output-package github.com/nakamasato/sample-controller/pkg/generated/listers --go-header-file /Users/m.naka/repos/kubernetes/code-generator/hack/boilerplate.go.txt --trim-path-prefix github.com/nakamasato/sample-controller
```
5. informer-gen:
```shell
${gobin}/informer-gen --input-dirs github.com/nakamasato/sample-controller/pkg/apis/example.com/v1alpha1 --versioned-clientset-package github.com/nakamasato/sample-controller/pkg/generated/clientset/versioned --listers-package github.com/nakamasato/sample-controller/pkg/generated/listers --output-package github.com/nakamasato/sample-controller/pkg/generated/informers --go-header-file /Users/m.naka/repos/kubernetes/code-generator/hack/boilerplate.go.txt --trim-path-prefix github.com/nakamasato/sample-controller
```
- コマンド実行の結果下記ファイルが生成される
  - `pkg/apis/example.com/v1alpha1/zz_generated.deepcopy.go`： この中ではFoo, FooListに対してDeepCopyオブジェクトが生成されている
  - `pkg/generated/`
    - `clientset`
    - `informers`
    - `listers`
- パッケージの更新
```shell
go mod tidy
```

###
#### CRDファイルの作成
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.example.com
spec:
  group: example.com  # apisディレクトリ配下の名前と一致
  names:
    kind: Foo
    listKind: FooList
    plural: foos  # 複数系
    singular: foo # 単数系
  scope: Namespaced  # クラスタースコープのリソースを作りたい場合はCluster
  versions:
    - name: v1alpha1 # apisディレクトリ配下の名前と一致
      served: true   # 提供するか
      storage: true  # 複数のバージョンがあった場合にどのバージョンを使ってetcdに保存するか決める。storage:trueのものが保存される
      schema:
        openAPIV3Schema:
          type: object  # この(openAPIV3Schema)中はObjectですっていう宣言。propertiesの中身がopenAPIV3Schemaを構成するkey名とvalueの型を宣言している
          properties:
            apiVersion:
              type: string
            kind:
              type: string
            metadata:
              type: object
            spec:
              type: object
              properties:
                deploymentName:  # specにdeploymentNameというプロパティを宣言
                  type: string
                replicas: # specにreplicasというプロパティを宣言
                  type: integer
                  minimum: 1 # 最小値
                  maximum: 10 # 最大値
            status:
              type: object
              properties:
                availableReplicas: # statusにavailableReplicasというプロパティを宣言
                  type: integer
```

###
#### 作成したtypesやclientset, CRD等の動作を確認
- main.goの実装
```go
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
```
- build
```shell
go build
```
- CRDの登録
```shell
kubectl apply -f config/crd/foos.yaml
```
- サンプル用のCR作成
```yaml
apiVersion: example.com/v1alpha1
kind: Foo
metadata:
    name: foo-sample
spec:
    deploymentName: foo-sample
    replicas: 1
```
```shell
kubectl apply -f config/sample/foo.yaml
```
- buildしたバイナリの実行
```shell
./clientgo-foo-controller
```

###
#### Controllerの実装（informerからhandlerを受け取る + Controllerを実行するためのRUNメソッドの作成）
- rootディレクトリに`controller.go`の作成
- controller.goにController構造体を作成
```go
type Controller struct {
    sampleclientset clientset.Interface
    fooSynced cache.InformerSynced // informerの中にあるキャッシュがsyncされているかどうかを判定する関数
}
```
- NewController関数の定義
```go
func NewInformer(sampleClient clientset.Interface, fooInformer informers.FooInformer) *Controller {
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

func (c *Controller) handleAdd(obj interface{}) {
	klog.Info("handleAdd is called")
}

func (c *Controller) handleDelete(obj interface{}) {
	klog.Info("handleDelete is called")
}
```
- Controller呼び出し元（main.go）の作成  
main関数に下記コードの追加
```go
// informerの作成
// informerはAPIサーバーをwatchしに行くのでclientsetが必要
// time.Second*30はinformerを30秒に一回resyncし直す
exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)
stopCh := make(chan struct{})
// controllerの作成
controller := NewController(exampleClient, exampleInformerFactory.Example().V1alpha1().Foos())
// informerのAPIサーバーのwatch開始
exampleInformerFactory.Start(stopCh)
// controllerの実行
if err = controller.Run(stopCh); err != nil {
klog.Fatalf("error occurred when running controller %s", err.Error())
}
```
- Fooの作成、削除でhandlerが呼び出されることをログから確認
```shell
kubectl apply -f config/sample/foo.yaml
kubectl delete -f config/sample/foo.yaml
```

###
#### Fooオブジェクトの取得
Informerのhandler(handleAdd, handleDelete)でトリガーしたオブジェクトをworkqueueというqueueに入れる  
queueの中のアイテムを１つずつ処理する関数を作成し、Lister(CodeGenerateされたFooLister)を使ってオブジェクトを取得する
- Controller構造体にworkqueueとfooListerを追加
```go
type Controller struct {
	sampleClient clientset.Interface
	foosSynced   cache.InformerSynced // Informerの中にあるキャッシュがsyncされているかどうかを判定する関数
	foosLister   listers.FooLister
	workqueue    workqueue.RateLimitingInterface
}
```
- Controller初期化処理にworkqueueとfooListerを追加
```go
controller := &Controller{
		sampleClient: sampleClient,
		foosSynced:   fooInformer.Informer().HasSynced,
		foosLister:   fooInformer.Lister(),
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "foo"),
	}
```
- handler（handleAdd, handleDelete）でトリガーしたオブジェクトをworkqueueに追加
```go
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
```
- queueの中から１つずつアイテムを取り出す
```go
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
```
- Controllerのrunworker（ControllerがRunすると終了するまで１秒毎に実行される関数）で呼び出し
```go
func (c *Controller) runWorker() {
    for c.processNextWorkItem() {
    }
}
```
- 実行確認
```shell
go run .
kubectl apply -f config/sample/foo.yaml
kubectl delete -f config/sample/foo.yaml
```

###
#### Fooリソースに対応するDeploymentの作成、削除
Deployment用のInformerを取得  
Fooに対応するDeploymentが存在するか確認  
存在しなかったらclientsetを使ってDeploymentを作成する  
Deploymentはbuilt-inリソースなのでInformerやLister、clientsetなどはimportして使用する

- ControllerにDeployment用のInformerとListerを作成
```go
type Controller struct {
	// 標準clientset
	kubeclientset kubernetes.Interface
	// カスタムリソース用のclientset
	sampleClient     clientset.Interface
	deploymentSynced cache.InformerSynced
	deploymentLister appslisters.DeploymentLister
```
- Controller初期化処理への反映
```go
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
```
- 下記をimportに追加
```go
appsinformers "k8s.io/client-go/informers/apps/v1"
"k8s.io/client-go/kubernetes"
appslisters "k8s.io/client-go/listers/apps/v1"
```
- main.go（controllerを使う側）にkubeclientの作成処理を追加
```go
+ kubeclient, err := kubernetes.NewForConfig(config)
+ if err != nil {
+     klog.Fatalf("Getting kubernetes client set %s", err.Error())
+ }

// configを使用してclientsetを取得
// このclientsetはCode Generatorを使用して作成したFooリソースを扱うことのできるclientset
exampleClient, err := clientset.NewForConfig(config)
if err != nil {
    klog.Fatalf("Error building example clientset: %s", err.Error())
}
```
- main.go（controllerを使う側）にkubeInformerFactoryの作成処理を追加
```go
+ kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)
```
- main.go（controllerを使う側）でkubeInformerFactoryのスタート処理の作成とController初期化処理の呼び出し引数を修正
```go
controller := NewController(
       kubeClient,
       exampleClient,
       kubeInformerFactory.Apps().V1().Deployments(),
       exampleInformerFactory.Example().V1alpha1().Foos(),
)
kubeInformerFactory.Start(stopCh)
```
- importの追加
```go
kubeinformers "k8s.io/client-go/informers"
"k8s.io/client-go/kubernetes"
```
- `controller.go`のrunWorkerでqueueを監視する関数（`processNextWorkItem`）に、「対応するDeploymentがあるか、なかったら作成する」というロジックを追加する
- `processNextWorkItem`中のkeyに対する処理を切り出した関数を追加
```go
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
```
- Deployment生成のためのappsv1.Deploymentを作成する関数を追加
```go
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
```
- `processNextWorkItem`の中からsyncHandlerを呼び出すように修正
```go
-               ns, name, err := cache.SplitMetaNamespaceKey(key)
-               if err != nil {
-                       klog.Errorf("failed to split key into namespace and name %s", err.Error())
-                       return err
+               if err := c.syncHandler(key); err != nil {
+                       // RateLimitがOKって言った時にqueueにアイテムを戻す（時間を置いて再度処理する）
+                       c.workqueue.AddRateLimited(key)
+                       return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
                }

-               // temporary main logic
-               foo, err := c.foosLister.Foos(ns).Get(name)
-               if err != nil {
-                       klog.Errorf("failed to get foo resource from lister %s", err.Error())
-                       return err
-               }
-               klog.Infof("Got foo %+v", foo.Spec)
-
```
- import更新
```go
import (
    "context"
    "fmt"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    samplev1alpha1 "github.com/nakamasato/sample-controller/pkg/apis/example.com/v1alpha1"
)
```
- newDeploymentでOwnerReferenceを設定しているのでFooが削除されるとDeploymentも削除される。DeleteFuncで処理する必要はないのでDeleteFuncのhandlerを削除しておく
```go
fooInformer.Informer().AddEventHandler(
      cache.ResourceEventHandlerFuncs{
              AddFunc:    controller.handleAdd,
-             DeleteFunc: controller.handleDelete,
      },
)

-func (c *Controller) handleDelete(obj interface{}) {
-       klog.Info("handleDelete is called")
-       c.enqueueFoo(obj)
-}
```
- 実行して確認
```shell
go run .
kubectl apply -f config/sample/foo.yaml
kubectl get deployment
kubectl delete -f config/sample/foo.yaml
kubectl get deployment
```

###
#### Fooオブジェクトを更新されたらDeploymentも更新同期する
FooオブジェクトのInformer#AddEventHandlerにUpdateFuncを追加  
FooオブジェクトのSpecで設定されている値と対応するDeploymentに設定されている値が異なる場合は、kubeclientsetでDeploymentを更新する  
- `controller.go`にconstメッセージの追加
```go
const (
    // MessageResourceExists is the message used for Events when a resource
    // fails to sync due to a Deployment already existing
    MessageResourceExists = "Resource %q already exists and is not managed by Foo"
)
```
- `controller.go syncHandler`に対象のdeploymentがcontrollerで管理されているものかの確認 + replicasの数を確認して、FooのSpecで指定しているものと異なる場合はdeploymentを更新するロジックを追加
```go
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
```
- FooリソースのInformerにUpdateFuncを追加
```go
// Informerにイベントハンドラの登録
fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: controller.enqueueFoo,
    UpdateFunc: func(oldObj, newObj interface{}) {
        controller.enqueueFoo(newObj)
    },
    // DeleteFunc: controller.handleDelete,
})
```
- 実行して確認
```shell
go run .
kubectl apply -f config/sample/foo.yaml
kubectl get deployment
// config/sample/foo.yamlのreplicasの値を更新
kubectl apply -f config/sample/foo.yaml
kubectl get deployment
kubectl delete -f config/sample/foo.yaml
```

###
#### Fooのステータスの更新
FooのステータスにDeploymentのavailableReplicasを格納する
- Fooのステータスを更新する関数を`controller.go`に追加
```go
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
```
- `controller.go syncHandler`にupdateFooStatusの呼び出しを追加
```go
// Finally, we update the status block of the Foo resource to reflect the
// current state of the world
err = c.updateFooStatus(foo, deployment)
if err != nil {
    klog.Errorf("failed to update Foo status for %s", foo.Name)
    return err
}
```
- CRD（`config/crd/foos.yaml`）のschemaと同じ階層にsubresourceのstatusを追加
https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#subresources
```yaml
subresources:
        status: {}
```
- 実行して確認（しばらく待たないとavailableReplicasに反映されない）
```shell
go run .
kubectl apply -f config/sample/foo.yaml
kubectl get deployment
kubectl get foo -o yaml
```

###
#### Deploymentの変更検知
DeploymentのavailableReplicasが更新されても、少し待たないとFooオブジェクトのavailableReplicasに反映されない問題があった。  
更新はFooInformerのResyncPeriodという機能に頼っていた。  
ResyncPeriodの更新は30秒に一回。つまり最大で30秒待たないとavailableReplicasは更新されなっかった。  
Deploymentが更新されたらすぐにFooオブジェクトのStatusが更新されるようにする。 
DeploymentInformerのAddFunc, UpdateFunc, DeleteFuncが実行された時にFooInformerと同じようにworkqueueにアイテムを追加していく。
- `controller.go`にhandleObject関数を追加
handleObjectはDeploymentからFooオブジェクトを取得してenqueueFooを呼び出す関数
```go
// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	// metav1のオブジェクトに変換できなかった場合にはDeletedFinalStateUnknown（キーとオブジェクトを持っている）に変換する
	// オブジェクトが削除された時に、Informerが持っているキャッシュの状態がわからなくなっていることがある
	// （APIサーバーからDisconnectしている間に削除されたオブジェクトのFinalStateがわからなくなっているような状態）
	// そのような場合にDeletedFinalStateUnknownに変換しようとする
	// DeletedFinalStateUnknownに変換できた場合には、DeletedFinalStateUnknownのオブジェクトを取り出して、metav1のオブジェクトに変換する
	// もし変換が可能であれば削除されたオブジェクトからリカバーした状態になる（objectに格納される）
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			return
		}
		klog.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.Infof("Processing object: %s", object.GetName())
	// objectのOwner Referenceを取得
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// Owner ReferenceのKindがFooであるか確認
		if ownerRef.Kind != "Foo" {
			return
		}

		// FooListerを使って、対象となるNameとNameSpaceにFooオブジェクトが存在しているのか確認する
		foo, err := c.foosLister.Foos(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.Errorf("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		// 取得できたFooをenqueueする
		c.enqueueFoo(foo)
		return
	}
}
```
- `controller.go NewController`にdeploymentInformerのAddEventHandlerを追加
```go
// Set up an event handler for when Deployment resources change. This
// handler will lookup the owner of the given Deployment, and if it is
// owned by a Foo resource then the handler will enqueue that Foo resource for
// processing. This way, we don't need to implement custom logic for
// handling Deployment resources. More info on this pattern:
// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: controller.handleObject,
    UpdateFunc: func(old, new interface{}) {
        // 渡されたobjectをDeploymentに変換
        newDepl := new.(*appsv1.Deployment)
        oldDepl := old.(*appsv1.Deployment)
        // リソースバージョンが一緒だったらリターン、異なっていればhandleObjectを実行
        if newDepl.ResourceVersion == oldDepl.ResourceVersion {
            // Resyncという仕組みを設定しているため、30秒に１回Update eventが呼ばれる
            // Resyncでイベントが呼ばれた場合にはリソースの変更があったわけではない可能性がある
            // リソースバージョンを見ることによって、実際に変更があったのか知ることができる
            // 実際に変更があった場合のみ後続の処理を行う
            return
        }
        controller.handleObject(new)
    },
    DeleteFunc: controller.handleObject,
})
```
- 実行して確認（PodがReadyになる分の遅延はあるが、即座に反映される）
```shell
go run .
kubectl apply -f config/sample/foo.yaml
kubectl get deployment
kubectl get foo -o yaml
// config/sample/foo.yamlのreplicasの値を更新
kubectl apply -f config/sample/foo.yaml
kubectl get foo -o yaml
```

###
#### Eventnの発火
kubernetesにはEventRecorderというEventを発火するための機能がある  
ControllerにEventRecorderを追加して、syncHandlerのなかでEventを発火する  
- Controllerにrecorderフィールドを追加
```go
type Controller struct {
	kubeclientset    kubernetes.Interface // 標準clientset
	sampleClient     clientset.Interface  // カスタムリソース用のclientset
	deploymentSynced cache.InformerSynced
	deploymentLister appslisters.DeploymentLister
	foosSynced       cache.InformerSynced // Informerの中にあるキャッシュがsyncされているかどうかを判定する関数
	foosLister       listers.FooLister
	workqueue        workqueue.RateLimitingInterface
	recorder         record.EventRecorder // EventRecorderはEventリソースをKubernetesAPIサーバーに記録するためのもの
}
```
- Controller初期化処理（NewController）でEventBroadcasterの作成とEventBroadcasterからEventRecorderの作成を行うロジックを追加
```go
eventBroadcaster := record.NewBroadcaster()
eventBroadcaster.StartStructuredLogging(0)
eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
controller := &Controller{
    kubeclientset:     kubeclientset,
    sampleclientset:   sampleclientset,
    foosLister:        fooInformer.Lister(),
    foosSynced:        fooInformer.Informer().HasSynced,
    workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "foo"),
    recorder:          recorder,
}
```
- constの更新
```go
const controllerAgentName = "clientgo-foo-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)
```
- importの更新
```go
+   "github.com/jpdel518/clientgo-foo-controller/pkg/generated/clientset/versioned/scheme"
    informers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/informers/externalversions/example.com/v1alpha1"
    listers "github.com/jpdel518/clientgo-foo-controller/pkg/generated/listers/example.com/v1alpha1"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/wait"
    appsinformers "k8s.io/client-go/informers/apps/v1"
    "k8s.io/client-go/kubernetes"
+   typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
    appslisters "k8s.io/client-go/listers/apps/v1"
    "k8s.io/client-go/tools/cache"
+   "k8s.io/client-go/tools/record"
```
- syncHandlerでEventの発火
```go
    // 対象のDeploymentがFooにコントロール（FooがオーナーのOwnerReferenceの関係にあるか）されているかどうかを確認
	// 対象のDeploymentがFooにコントロールされてるものでない場合はエラーを返す
	if !metav1.IsControlledBy(deployment, foo) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
+		c.recorder.Event(foo, corev1.EventTypeWarning, ErrResourceExists, msg)
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

+	c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
```
- 実行して確認
```shell
go run .
kubectl apply -f config/sample/foo.yaml
kubectl get event --field-selector involvedObject.kind=Foo
kubectl delete -f config/sample/foo.yaml

kubectl create deployment foo-sample --image nginx
kubectl apply -f config/sample/foo.yaml
kubectl get event --field-selector involvedObject.kind=Foo
kubectl delete -f config/sample/foo.yaml
kubectl delete deployment foo-sample
```
