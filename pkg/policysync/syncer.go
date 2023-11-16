package policysync

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

type Syncer interface {
	SyncPolicy(ctx context.Context, apiclient client.Client, policy Policy) error
}

type FakeSyncer struct {
}

var _ Syncer = &FakeSyncer{}

func (*FakeSyncer) SyncPolicy(ctx context.Context, _ client.Client, policy Policy) error {
	log := crlog.FromContext(ctx)

	targetRef := policy.GetTargetRef()
	log.Info("Syncing policy", "policy", policy, "targetRef", targetRef)

	return nil
}
