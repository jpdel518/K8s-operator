/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	passwordGenerator "github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	secretv1alpha1 "example.com/password-operator/api/v1alpha1" // api/v1alpha1/のパッケージをインポート
)

// PasswordReconciler reconciles a Password object
type PasswordReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=secret.example.com,resources=passwords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secret.example.com,resources=passwords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=secret.example.com,resources=passwords/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Password object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PasswordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconcile Password object", "password", req.NamespacedName)

	// Fetch Password object
	var password secretv1alpha1.Password
	if err := r.Get(ctx, req.NamespacedName, &password); err != nil {
		logger.Error(err, "Fetch Password object - failed")
		// client.IgnoreNotFound(err)は、NotFoundエラー（既に削除済みの場合）を無視するためのヘルパー関数
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Fetch Password object - succeeded", "password", password.Name, "createdAt", password.CreationTimestamp)

	// Create Secret object if not exists
	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			// Create Secret
			logger.Info("Create Secret object if not exists - create secret")
			// Generate a password that is 64 characters long with 10 digits, 10 symbols,
			// allowing upper and lower case letters, disallowing repeat characters.
			// passwordStr, err := passwordGenerator.Generate(64, 10, 10, false, false)
			// Specから各設定値を取得
			passwordStr, err := passwordGenerator.Generate(
				password.Spec.Length,
				password.Spec.Digit,
				password.Spec.Symbol,
				password.Spec.CaseSensitive,
				password.Spec.DisallowRepeat,
			)
			if err != nil {
				logger.Error(err, "Create Secret object if not exists - failed to generate password")

				// PasswordオブジェクトのstatusをFailedに更新
				password.Status.State = secretv1alpha1.PasswordFailed
				password.Status.Reason = "failed to generate password"
				if err := r.Status().Update(ctx, &password); err != nil {
					logger.Error(err, "Failed to update Password status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
			secret := newSecretFromPassword(&password, passwordStr)
			// Password Objectと作成するSecretの間にreferenceを作成
			// Password Objectが削除されたらSecretはガベージコレクタに削除される
			err = ctrl.SetControllerReference(&password, secret, r.Scheme) // Set owner of this Secret
			if err != nil {
				logger.Error(err, "Create Secret object if not exists - failed to set SetControllerReference")

				// PasswordオブジェクトのstatusをFailedに更新
				password.Status.State = secretv1alpha1.PasswordFailed
				password.Status.Reason = "failed to set SetControllerReference"
				if err := r.Status().Update(ctx, &password); err != nil {
					logger.Error(err, "Failed to update Password status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
			// 作成実行
			err = r.Create(ctx, secret)
			// Secret作成失敗
			if err != nil {
				logger.Error(err, "Create Secret object if not exists - failed to create Secret")

				// PasswordオブジェクトのstatusをInSyncに更新
				password.Status.State = secretv1alpha1.PasswordFailed
				password.Status.Reason = "failed to create Secret"
				if err := r.Status().Update(ctx, &password); err != nil {
					logger.Error(err, "Failed to update Password status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
			logger.Info("Create Secret object if not exists - Secret successfully created")
		} else {
			logger.Error(err, "Create Secret object if not exists - failed to fetch Secret")

			// PasswordオブジェクトのstatusをInSyncに更新
			password.Status.State = secretv1alpha1.PasswordFailed
			password.Status.Reason = "failed to fetch Secret"
			if err := r.Status().Update(ctx, &password); err != nil {
				logger.Error(err, "Failed to update Password status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}
	}

	logger.Info("Create Secret object if not exists - completed")

	// PasswordオブジェクトのstatusをInSyncに更新
	password.Status.State = secretv1alpha1.PasswordInSync
	if err := r.Status().Update(ctx, &password); err != nil {
		logger.Error(err, "Failed to update Password status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PasswordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&secretv1alpha1.Password{}).
		Complete(r)
}

func newSecretFromPassword(password *secretv1alpha1.Password, passwordStr string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      password.Name,
			Namespace: password.Namespace,
		},
		Data: map[string][]byte{
			"password": []byte(passwordStr),
		},
	}
	return secret
}