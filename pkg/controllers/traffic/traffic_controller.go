/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

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

package traffic

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kuadrantv1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	trafficFinalizer = "kuadrant.io/traffic-management"
)

// Reconciler reconciles a traffic object
type Reconciler struct {
	WorkloadClient client.Client
	Hosts          HostService
	Certificates   CertificateService
}

type HostService interface {
	EnsureManagedHost(ctx context.Context, t traffic.Interface) ([]string, []*kuadrantv1.DNSRecord, error)
	AddEndPoints(ctx context.Context, t traffic.Interface) error
	RemoveEndpoints(ctx context.Context, t traffic.Interface) error
}

type CertificateService interface {
	EnsureCertificate(ctx context.Context, host string, owner metav1.Object) error
	GetCertificateSecret(ctx context.Context, host string) (*v1.Secret, error)
}

func (r *Reconciler) Handle(ctx context.Context, o runtime.Object) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	trafficAccessor := o.(traffic.Interface)
	log.Log.Info("got traffic object", "kind", trafficAccessor.GetKind(), "name", trafficAccessor.GetName(), "namespace", trafficAccessor.GetNamespace())
	controllerutil.AddFinalizer(trafficAccessor, trafficFinalizer)
	// TODO add in deletion logic
	if trafficAccessor.GetDeletionTimestamp() != nil && !trafficAccessor.GetDeletionTimestamp().IsZero() {
		// targets, err := trafficAccessor.GetDNSTargets()
		if err := r.Hosts.RemoveEndpoints(ctx, trafficAccessor); err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(trafficAccessor, trafficFinalizer)
		return ctrl.Result{}, nil
	}

	// verify host is correct
	// no managed host assigned assign one
	// create empty DNSRecord with assigned host
	managedHosts, records, err := r.Hosts.EnsureManagedHost(ctx, trafficAccessor)
	if err != nil && err != dns.AlreadyAssignedErr {
		return ctrl.Result{}, err
	}
	for i, managedHost := range managedHosts {
		record := records[i]
		log.Log.Info("managed record ", "record", managedHost)
		if err := trafficAccessor.AddManagedHost(managedHost); err != nil {
			return ctrl.Result{}, err
		}
		// create certificate resource for assigned host
		log.Log.Info("host assigned ensuring certificate in place")
		if err := r.Certificates.EnsureCertificate(ctx, managedHost, record); err != nil && !k8serrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		// when certificate ready copy secret (need to add event handler for certs)
		// only once certificate is ready update DNS based status of ingress
		secret, err := r.Certificates.GetCertificateSecret(ctx, managedHost)
		if err != nil && !k8serrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		// if err is not exists return and wait
		if err != nil {
			log.Log.Info("tls secret does not exist yet for host " + managedHost + " requeue")
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
		}
		log.Log.Info("certificate exists for host", "host", managedHost)

		//copy secret
		if secret != nil {
			if err := r.copySecretToWorkloadCluster(ctx, trafficAccessor, secret, managedHost); err != nil {
				return ctrl.Result{}, err
			}
			trafficAccessor.AddTLS(managedHost, secret)
		}
		log.Log.Info("certificate secret in place for  host adding dns endpoints", "host", managedHost)
		if err := r.Hosts.AddEndPoints(ctx, trafficAccessor); err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) copySecretToWorkloadCluster(ctx context.Context, trafficAccessor traffic.Interface, tls *v1.Secret, host string) error {
	log.Log.Info(fmt.Sprintf("tls secret ready for host %s. copying secret", host))
	copySecret := tls.DeepCopy()
	copySecret.ObjectMeta = metav1.ObjectMeta{
		Name:      host,
		Namespace: trafficAccessor.GetNamespace(),
	}
	if err := r.WorkloadClient.Create(ctx, copySecret, &client.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			if err := r.WorkloadClient.Get(ctx, client.ObjectKeyFromObject(copySecret), copySecret); err != nil {
				return err
			}
			copySecret.Data = tls.Data
			if err := r.WorkloadClient.Update(ctx, copySecret, &client.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}
