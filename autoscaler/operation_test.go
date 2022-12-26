package autoscaler

import (
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticcloud"
	"testing"
)

func TestScalingOperation_Direction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		op   *ScalingOperation
		want ScalingDirection
	}{
		{
			name: "scaling out",
			op: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(64),
				ToTopologySize:   elasticcloud.NewTopologySize(128),
			},
			want: ScalingDirectionOut,
		},
		{
			name: "scaling in",
			op: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(128),
				ToTopologySize:   elasticcloud.NewTopologySize(64),
			},
			want: ScalingDirectionIn,
		},
		{
			name: "not scaling",
			op: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(64),
				ToTopologySize:   elasticcloud.NewTopologySize(64),
			},
			want: ScalingDirectionNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.op.Direction(); got != tt.want {
				t.Errorf("Direction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScalingOperation_NeedTopologySizeUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		op   *ScalingOperation
		want bool
	}{
		{
			name: "update is needed",
			op: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(64),
				ToTopologySize:   elasticcloud.NewTopologySize(128),
			},
			want: true,
		},
		{
			name: "update is not needed",
			op: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(64),
				ToTopologySize:   elasticcloud.NewTopologySize(64),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.op.NeedTopologySizeUpdate(); got != tt.want {
				t.Errorf("NeedTopologySizeUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}
