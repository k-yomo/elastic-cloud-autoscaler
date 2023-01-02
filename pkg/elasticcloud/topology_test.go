package elasticcloud

import (
	"reflect"
	"testing"

	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/util/ec"
)

func TestNewTopologySize(t *testing.T) {
	t.Parallel()

	type args struct {
		gib int
	}
	tests := []struct {
		name string
		args args
		want *models.TopologySize
	}{
		{
			name: "initializes topology size with given GiB",
			args: args{
				gib: 64,
			},
			want: &models.TopologySize{
				Resource: ec.String("memory"),
				Value:    ec.Int32(65536), // 64 * 1024
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTopologySize(tt.args.gib); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTopologySize() = %v, want %v", got, tt.want)
			}
		})
	}
}
