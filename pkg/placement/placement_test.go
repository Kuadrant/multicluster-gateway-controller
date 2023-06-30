//go:build unit

package placement_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	pd "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
)

func init() {
	if err := workv1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	if err := pd.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

func TestGetAddresses(t *testing.T) {
	testAddress := "127.0.0.1"

	cases := []struct {
		Name              string
		Gateway           *v1beta1.Gateway
		DownstreamCluster string
		ManifestWork      func(downstream, name string) *workv1.ManifestWork
		Assert            func(t *testing.T, err error, address []v1beta1.GatewayAddress)
	}{
		{
			Name:              "test get addresses returns remote ip when status present",
			DownstreamCluster: "test",
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			ManifestWork: func(downstream, name string) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
					},
					Status: workv1.ManifestWorkStatus{
						ResourceStatus: workv1.ManifestResourceStatus{
							Manifests: []workv1.ManifestCondition{
								{
									StatusFeedbacks: workv1.StatusFeedbackResult{
										Values: []workv1.FeedbackValue{{
											Name: "addresses",
											Value: workv1.FieldValue{
												String: &testAddress,
											},
										}},
									},
									ResourceMeta: workv1.ManifestResourceMeta{
										Group: "gateway.networking.k8s.io",
										Name:  "test",
									},
								},
							},
						},
					},
				}
			},
			Assert: func(t *testing.T, err error, address []v1beta1.GatewayAddress) {
				if err != nil {
					t.Fatalf("did not expect an error but got %s", err)
				}
				if address == nil || len(address) != 1 {
					t.Fatalf("expected 1 address to be returned but got %v", address)
				}
				if address[0].Value != testAddress {
					t.Fatalf("expected address to be %s but got %s", testAddress, address[0].Value)
				}
			},
		},
		{
			Name:              "test get addresses returns none when no status present",
			DownstreamCluster: "test",
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			ManifestWork: func(downstream, name string) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
					},
					Status: workv1.ManifestWorkStatus{
						ResourceStatus: workv1.ManifestResourceStatus{},
					},
				}
			},
			Assert: func(t *testing.T, err error, address []v1beta1.GatewayAddress) {
				if err != nil {
					t.Fatalf("expected no error but got one %s", err)
				}
				if address == nil || len(address) != 0 {
					t.Fatalf("expected 0 address to be returned but got %v", address)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithObjects(tc.ManifestWork(tc.DownstreamCluster, placement.WorkName(tc.Gateway))).Build()
			p := placement.NewOCMPlacer(f)
			addr, err := p.GetAddresses(context.TODO(), tc.Gateway, tc.DownstreamCluster)
			tc.Assert(t, err, addr)
		})
	}

}

func TestListenerTotalAttachedRoutes(t *testing.T) {
	cases := []struct {
		Name               string
		Gateway            *v1beta1.Gateway
		DownstreamCluster  string
		AttachedRouteCount int64
		ManifestWork       func(downstream, name string, routes int64) *workv1.ManifestWork
		Assert             func(t *testing.T, err error, actual, expected int64)
	}{
		{
			Name:               "test total attached routes return correct number",
			DownstreamCluster:  "test",
			AttachedRouteCount: 1,
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			ManifestWork: func(downstream, name string, attachedRoutes int64) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
					},
					Status: workv1.ManifestWorkStatus{
						ResourceStatus: workv1.ManifestResourceStatus{
							Manifests: []workv1.ManifestCondition{
								{
									ResourceMeta: workv1.ManifestResourceMeta{
										Group: "gateway.networking.k8s.io",
										Name:  "test",
									},
									StatusFeedbacks: workv1.StatusFeedbackResult{
										Values: []workv1.FeedbackValue{
											{
												Name: "listenerapiattachedroutes",
												Value: workv1.FieldValue{
													Integer: &attachedRoutes,
												},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			Assert: func(t *testing.T, err error, actualTotal, expectedTotal int64) {
				if err != nil {
					t.Fatalf("did not expect an error but got one %s ", err)
				}
				if actualTotal != expectedTotal {
					t.Fatalf("the expected total %v did not match the actual total %v", expectedTotal, actualTotal)
				}
			},
		},
		{
			Name:               "test total attached routes return 0 and error when no status",
			DownstreamCluster:  "test",
			AttachedRouteCount: 0,
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			ManifestWork: func(downstream, name string, attachedRoutes int64) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
					},
					Status: workv1.ManifestWorkStatus{
						ResourceStatus: workv1.ManifestResourceStatus{
							Manifests: []workv1.ManifestCondition{},
						},
					},
				}
			},
			Assert: func(t *testing.T, err error, actualTotal, expectedTotal int64) {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				if actualTotal != expectedTotal {
					t.Fatalf("the expected total %v did not match the actual total %v", expectedTotal, actualTotal)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			f := fake.NewClientBuilder().
				WithObjects(tc.ManifestWork(tc.DownstreamCluster, placement.WorkName(tc.Gateway), tc.AttachedRouteCount)).
				Build()
			p := placement.NewOCMPlacer(f)
			total, err := p.ListenerTotalAttachedRoutes(context.TODO(), tc.Gateway, "api", tc.DownstreamCluster)
			tc.Assert(t, err, int64(total), int64(tc.AttachedRouteCount))
		})
	}
}

func TestGetPlacedClusters(t *testing.T) {
	cases := []struct {
		Name               string
		ManifestWork       func(downstream, name string) *workv1.ManifestWork
		Gateway            *v1beta1.Gateway
		DownstreamClusters []string
		Assert             func(t *testing.T, err error, cluster sets.Set[string], downstreams []string)
	}{
		{
			Name: "test placed clusters returned",
			ManifestWork: func(downstream, name string) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
						Labels: map[string]string{
							placement.WorkManifestLabel: name,
						},
					},
					Status: workv1.ManifestWorkStatus{
						Conditions: []v1.Condition{
							{
								Type:   workv1.WorkApplied,
								Status: v1.ConditionTrue,
							},
						},
					},
				}
			},
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			DownstreamClusters: []string{"test", "other"},
			Assert: func(t *testing.T, err error, clusters sets.Set[string], downstreams []string) {
				if err != nil {
					t.Fatalf("did not expect an error but got one %s", err)
				}
				if nil == clusters || clusters.Len() != len(downstreams) {
					t.Fatalf("expected the gateway to be placed on %v  clusters but got %v", len(downstreams), clusters.Len())
				}
			},
		},
		{
			Name: "test no clusters returned when not yet placed on chosen clusters",
			ManifestWork: func(downstream, name string) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
						Labels: map[string]string{
							placement.WorkManifestLabel: name,
						},
					},
					Status: workv1.ManifestWorkStatus{
						Conditions: []v1.Condition{},
					},
				}
			},
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			DownstreamClusters: []string{"test", "other"},
			Assert: func(t *testing.T, err error, clusters sets.Set[string], downstreams []string) {
				if err != nil {
					t.Fatalf("did not expect an error but got one %s", err)
				}
				if nil == clusters || clusters.Len() != 0 {
					t.Fatalf("expected the gateway to be placed on %v  clusters but got %v", 0, clusters.Len())
				}
			},
		},
		{
			Name: "test no clusters returned when no downstreams yet",
			ManifestWork: func(downstream, name string) *workv1.ManifestWork {
				return &workv1.ManifestWork{
					ObjectMeta: v1.ObjectMeta{
						Name:      name,
						Namespace: downstream,
						Labels: map[string]string{
							placement.WorkManifestLabel: name,
						},
					},
					Status: workv1.ManifestWorkStatus{
						Conditions: []v1.Condition{},
					},
				}
			},
			Gateway: &v1beta1.Gateway{
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			DownstreamClusters: []string{},
			Assert: func(t *testing.T, err error, clusters sets.Set[string], downstreams []string) {
				if err != nil {
					t.Fatalf("did not expect an error but got one %s", err)
				}
				if nil == clusters || clusters.Len() != 0 {
					t.Fatalf("expected the gateway to be placed on %v  clusters but got %v", 0, clusters.Len())
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			f := fake.NewClientBuilder()
			for _, ds := range tc.DownstreamClusters {
				f = f.WithObjects(tc.ManifestWork(ds, placement.WorkName(tc.Gateway)))
			}

			p := placement.NewOCMPlacer(f.Build())
			placed, err := p.GetPlacedClusters(context.TODO(), tc.Gateway)
			tc.Assert(t, err, placed, tc.DownstreamClusters)
		})
	}

}

func TestGetClusters(t *testing.T) {
	cases := []struct {
		Name              string
		PlacementDecision func(clusters sets.Set[string]) *pd.PlacementDecision
		Gateway           *v1beta1.Gateway
		Clusters          sets.Set[string]
		Assert            func(t *testing.T, err error, clusters, expected sets.Set[string])
	}{
		{
			Name:     "test all targeted clusters returned",
			Clusters: sets.Set[string](sets.NewString("c1", "c2")),
			Gateway: &v1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Labels:    map[string]string{placement.OCMPlacementLabel: "test"},
					Namespace: "test",
				},
			},
			Assert: func(t *testing.T, err error, got, expected sets.Set[string]) {
				if err != nil {
					t.Fatalf("did not expect an error but got one %s", err)
				}
				if !got.Equal(expected) {
					t.Fatalf("expected clusters %v but it was not present in %v", expected.UnsortedList(), got.UnsortedList())
				}
			},
			PlacementDecision: func(clusters sets.Set[string]) *pd.PlacementDecision {

				dec := &pd.PlacementDecision{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							placement.OCMPlacementLabel: "test",
						},
						Namespace: "test",
					},
					Status: pd.PlacementDecisionStatus{},
				}
				for _, c := range clusters.UnsortedList() {
					dec.Status.Decisions = append(dec.Status.Decisions, pd.ClusterDecision{
						ClusterName: c,
					})
				}
				return dec
			},
		},
		{
			Name:     "test no clusters returned when no  matching placement decision",
			Clusters: sets.Set[string](sets.NewString()),
			Gateway: &v1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Labels:    map[string]string{placement.OCMPlacementLabel: "test"},
					Namespace: "test",
				},
			},
			Assert: func(t *testing.T, err error, got, expected sets.Set[string]) {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				if !got.Equal(expected) {
					t.Fatalf("expected clusters %v but it was not present in %v", expected.UnsortedList(), got.UnsortedList())
				}
			},
			PlacementDecision: func(clusters sets.Set[string]) *pd.PlacementDecision {
				dec := &pd.PlacementDecision{
					Status: pd.PlacementDecisionStatus{},
				}
				return dec
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			f := fake.NewClientBuilder()
			f.WithObjects(tc.PlacementDecision(tc.Clusters))
			p := placement.NewOCMPlacer(f.Build())
			cs, err := p.GetClusters(context.TODO(), tc.Gateway)
			tc.Assert(t, err, cs, tc.Clusters)
		})
	}
}

func TestDeschedule(t *testing.T) {
	var manifestWorkFunc = func(downstream, name string) *workv1.ManifestWork {

		return &workv1.ManifestWork{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: downstream,
				Labels:    map[string]string{placement.WorkManifestLabel: name},
			},
			Status: workv1.ManifestWorkStatus{
				Conditions: []v1.Condition{
					{
						Type:   workv1.WorkApplied,
						Status: metav1.ConditionTrue,
					},
				},
			},
		}

	}

	var placementDecisionFunc = func(clusters sets.Set[string]) *pd.PlacementDecision {

		dec := &pd.PlacementDecision{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					placement.OCMPlacementLabel: "test",
				},
				Namespace: "test",
				Name:      "test",
			},
			Status: pd.PlacementDecisionStatus{},
		}
		for _, c := range clusters.UnsortedList() {
			dec.Status.Decisions = append(dec.Status.Decisions, pd.ClusterDecision{
				ClusterName: c,
			})
		}
		return dec
	}

	var commonAssert = func(t *testing.T, currentTarget string, manifests *workv1.ManifestWorkList, err error) {
		if err != nil {
			t.Fatalf("did not expect an error but got one %s", err)
		}
		// we expect two manifests per gateway (1 for rbac one for the gateway)
		if len(manifests.Items) != 2 {
			t.Fatalf("unexpected number of manifests %v", len(manifests.Items))
		}
		rbacFound := false
		gatewayFound := false
		for _, m := range manifests.Items {
			if m.Namespace != currentTarget {
				t.Fatalf("expected the manifests to be in the cluster namespace")
			}
			if m.Name == "gateway-rbac" {
				rbacFound = true
			}
			if m.Name == "gateway-test-test" {
				gatewayFound = true
			}
		}
		if !rbacFound {
			t.Fatalf("expected an rbac manifest but got none")
		}
		if !gatewayFound {
			t.Fatalf("expected to find a gateway but got none")
		}
	}

	cases := []struct {
		Name              string
		Upstream          *v1beta1.Gateway
		Downstream        *v1beta1.Gateway
		TLSSecrets        []v1.Object
		PlacementDecision func(clusters sets.Set[string]) *pd.PlacementDecision
		ManifestWork      func(downstream, name string) *workv1.ManifestWork
		// where should it exist
		Clusters sets.Set[string]
		// where does it currently exist
		Existing sets.Set[string]
		Assert   func(t *testing.T, currentTarget string, manifests *workv1.ManifestWorkList, err error)
	}{
		{
			Name: "test gateway created in correct clusters",
			Upstream: &v1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Labels:    map[string]string{placement.OCMPlacementLabel: "test"},
					Namespace: "test",
					Name:      "test",
				},
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
			},
			Downstream: &v1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
				},
				TypeMeta: v1.TypeMeta{
					Kind:       "Gateway",
					APIVersion: "gateway.networking.k8s.io/v1beta1",
				},
			},
			Existing:          sets.Set[string](sets.NewString()),
			Clusters:          sets.Set[string](sets.NewString("c1", "c2")),
			PlacementDecision: placementDecisionFunc,
			TLSSecrets: []v1.Object{
				&corev1.Secret{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			ManifestWork: manifestWorkFunc,
			Assert:       commonAssert,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var f = fake.NewClientBuilder()
			placedesc := tc.PlacementDecision(tc.Clusters)
			f.WithObjects(placedesc)
			for _, ds := range tc.Existing.UnsortedList() {
				mfs := tc.ManifestWork(ds, placement.WorkName(tc.Upstream))
				f = f.WithObjects(mfs)
			}
			c := f.Build()
			p := placement.NewOCMPlacer(c)
			// build a test function as we want to change state and execute twice

			placed, err := p.Place(context.TODO(), tc.Upstream, tc.Downstream, tc.TLSSecrets...)
			if placed != nil && !placed.Equal(tc.Clusters) {
				t.Fatalf("expected placed clusters %v to equal the target clusters %v", placed.UnsortedList(), tc.Clusters.UnsortedList())
			}
			l := &workv1.ManifestWorkList{}
			if err := c.List(context.TODO(), l, &client.ListOptions{}); err != nil {
				t.Fatalf("did not expect an error listing manifests but got one %s", err)
			}

			// multiply by 2 as we expect and rbac and gatway manifest
			if len(l.Items) != tc.Clusters.Len()*2 {
				t.Fatalf("expected there to be %v manifests but got %v", tc.Clusters.Len()*2, len(l.Items))
			}

			for _, target := range tc.Clusters.UnsortedList() {
				// retrieve and validate the manifest for each cluster
				if err := c.List(context.TODO(), l, &client.ListOptions{Namespace: target}); err != nil {
					t.Fatalf("did not expect an error listing manifests but got one %s", err)
				}

				tc.Assert(t, target, l, err)
			}
		})
	}
}
