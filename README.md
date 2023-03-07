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
