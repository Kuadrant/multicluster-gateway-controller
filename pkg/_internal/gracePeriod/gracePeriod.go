package gracePeriod

import (
	"context"
	"fmt"
	"strconv"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	GraceTimestampAnnotation = "kuadrant.io/grace-timeout"
	DefaultGracePeriod       = time.Second * dns.DefaultTTL * 10
)

var ErrGracePeriodNotExpired = fmt.Errorf("grace period has not yet expired")

func GracefulDelete(ctx context.Context, c client.Client, obj client.Object, ignoreGrace bool) error {
	log := log.Log
	at := time.Now().Add(DefaultGracePeriod)
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		log.V(3).Info("error finding object to graceful delete")
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// If ignored the grace, of if At is before the current time, just delete it
	if ignoreGrace || at.Before(time.Now()) || at.Equal(time.Now()) {
		if err := c.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
			return err
		}
		return nil
	}

	//ensure annotation is present
	if !metadata.HasAnnotation(obj, GraceTimestampAnnotation) {
		log.V(3).Info("no grace annotation set, adding one now")
		metadata.AddAnnotation(obj, GraceTimestampAnnotation, strconv.FormatInt(at.Unix(), 10))
		if err := c.Update(ctx, obj); err != nil {
			return err
		}
		return ErrGracePeriodNotExpired
	}
	deleteAt, err := strconv.Atoi(obj.GetAnnotations()[GraceTimestampAnnotation])
	if err != nil {
		log.V(3).Info("existing grace annotation has bad value, resetting it")
		metadata.AddAnnotation(obj, GraceTimestampAnnotation, strconv.FormatInt(at.Unix(), 10))
		if err := c.Update(ctx, obj); err != nil {
			return err
		}
		return ErrGracePeriodNotExpired
	}

	//grace time reached, delete it
	if int64(deleteAt) <= time.Now().Unix() {
		log.V(3).Info("grace period expired, removing object")
		return c.Delete(ctx, obj)
	}

	log.V(3).Info("grace period still pending")

	return fmt.Errorf("%w expires at %v", ErrGracePeriodNotExpired, deleteAt)
}
