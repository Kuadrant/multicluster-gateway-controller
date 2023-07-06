package dns

import "testing"

func Test_toBase36hash(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"c1", "2piivc"},
		{"c2", "2pcjv8"},
		{"g1", "egzg90"},
		{"g2", "28bp8h"},
		{"cluster1", "20st0r"},
		{"cluster2", "1c80l6"},
		{"gateway1", "2hyvk7"},
		{"gateway2", "5c23wh"},
		{"prod-web-multi-cluster-gateways", "4ej5le"},
		{"kind-mgc-control-plane", "2c71gf"},
		{"test-cluster-1", "20qri0"},
		{"test-cluster-2", "2pj3we"},
		{"testgw-testns", "0ecjaw"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := ToBase36hash(tt.in); got != tt.want {
				t.Errorf("ToBase36hash() = %v, want %v", got, tt.want)
			}
		})
	}
}
