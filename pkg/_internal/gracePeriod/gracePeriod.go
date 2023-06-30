package gracePeriod

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	GraceTimestampAnnotation = "kuadrant.io/grace-timeout"
	DefaultGracePeriod       = time.Second * dns.DefaultTTL * 10
)

var ErrGracePeriodNotExpired = fmt.Errorf("grace period has not yet expired")

func WasGracePeriodNotExpiredErr(e error) bool {
	return strings.Contains(e.Error(), "grace period has not yet expired")
}

func GracefulDelete(ctx context.Context, c client.Client, obj client.Object) error {
	at := time.Now().Add(DefaultGracePeriod)
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return err
	}

	// if At is before the current time, just delete it
	if at.Before(time.Now()) || at.Equal(time.Now()) {
		return c.Delete(ctx, obj)
	}

	//ensure finalizer and annotation are present
	if !metadata.HasAnnotation(obj, GraceTimestampAnnotation) {
		metadata.AddAnnotation(obj, GraceTimestampAnnotation, strconv.FormatInt(at.Unix(), 10))
		return c.Update(ctx, obj)
	}
	deleteAt, err := strconv.Atoi(obj.GetAnnotations()[GraceTimestampAnnotation])
	if err != nil {
		//badly formed deleteAt annotation, remove it, so it will be regenerated
		metadata.RemoveAnnotation(obj, GraceTimestampAnnotation)
		if err := c.Update(ctx, obj); err != nil {
			return err
		}
	} else {
		//grace time reached, delete it
		if int64(deleteAt) <= time.Now().Unix() {
			return c.Delete(ctx, obj)
		}
	}
	return ErrGracePeriodNotExpired
}
