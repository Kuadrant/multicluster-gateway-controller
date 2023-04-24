package traffic

import (
	"context"
	"reflect"
	"testing"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	status "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestGateway_GetDNSTargets(t *testing.T) {
	type fields struct {
		Gateway *gatewayv1beta1.Gateway
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []v1alpha1.Target
		wantErr bool
	}{
		{
			name: "Generates an IP target",
			fields: fields{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test", Namespace: "test",
						Annotations: map[string]string{
							status.SyncerClusterStatusAnnotationPrefix + "-cluster": "{\"addresses\":[{\"type\":\"IPAddress\",\"value\":\"127.0.0.1\"}],\"conditions\":[],\"listeners\":[]}",
						},
					},
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: []v1alpha1.Target{{
				TargetType: v1alpha1.TargetTypeIP,
				Value:      "127.0.0.1",
			}},
			wantErr: false,
		},
		{
			name: "Generates a HOST target",
			fields: fields{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test", Namespace: "test",
						Annotations: map[string]string{
							status.SyncerClusterStatusAnnotationPrefix + "-cluster": "{\"addresses\":[{\"type\":\"Hostname\",\"value\":\"this.is.a.host\"}],\"conditions\":[],\"listeners\":[]}",
						},
					},
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: []v1alpha1.Target{{
				TargetType: v1alpha1.TargetTypeHost,
				Value:      "this.is.a.host",
			}},
			wantErr: false,
		},
		{
			name: "Detects a hostname incorrectly marked as an IP",
			fields: fields{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test", Namespace: "test",
						Annotations: map[string]string{
							status.SyncerClusterStatusAnnotationPrefix + "-cluster": "{\"addresses\":[{\"type\":\"IPAddress\",\"value\":\"this.is.a.host\"}],\"conditions\":[],\"listeners\":[]}",
						},
					},
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: []v1alpha1.Target{{
				TargetType: v1alpha1.TargetTypeHost,
				Value:      "this.is.a.host",
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Gateway{
				Gateway: tt.fields.Gateway,
			}
			got, err := a.GetDNSTargets(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Gateway.GetDNSTargets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Gateway.GetDNSTargets() = %v, want %v", got, tt.want)
			}
		})
	}
}
