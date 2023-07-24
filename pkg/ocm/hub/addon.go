package hub

import (
	"fmt"

	"open-cluster-management.io/addon-framework/pkg/agent"
	workapiv1 "open-cluster-management.io/api/work/v1"
)

func AddonHealthProber() *agent.HealthProber {
	return &agent.HealthProber{
		Type: agent.HealthProberTypeWork,
		WorkProber: &agent.WorkHealthProber{
			ProbeFields: []agent.ProbeField{
				{
					ResourceIdentifier: workapiv1.ResourceIdentifier{
						Group:     "operators.coreos.com",
						Resource:  "subscriptions",
						Name:      "kuadrant-operator",
						Namespace: "operators",
					},
					ProbeRules: []workapiv1.FeedbackRule{
						{
							Type: workapiv1.JSONPathsType,
							JsonPaths: []workapiv1.JsonPath{
								{
									Name:    "healthy",
									Path:    ".status.catalogHealth[0].healthy",
									Version: "v1alpha1",
								},
							},
						},
					},
				},
			},
			HealthCheck: func(ri workapiv1.ResourceIdentifier, sfr workapiv1.StatusFeedbackResult) error {
				if len(sfr.Values) == 0 {
					return fmt.Errorf("nothing for health prober for deployment %s/%s", ri.Namespace, ri.Name)
				}
				for _, value := range sfr.Values {
					if value.Name != "healthy" {
						continue
					}

					if *value.Value.Boolean {
						return nil
					}

					return fmt.Errorf("readyReplica is %d for deployment %s/%s", *value.Value.Integer, ri.Namespace, ri.Name)
				}
				return fmt.Errorf("readyReplica is not probed")
			},
		},
	}
}
