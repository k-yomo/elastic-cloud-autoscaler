package elasticcloud

import (
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/util/ec"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/memory"
	"reflect"
	"testing"
)

func TestFindHotContentTopology(t *testing.T) {
	t.Parallel()

	type args struct {
		topologies []*models.ElasticsearchClusterTopologyElement
	}
	tests := []struct {
		name string
		args args
		want *models.ElasticsearchClusterTopologyElement
	}{
		{
			name: "returns hot content topology",
			args: args{
				topologies: []*models.ElasticsearchClusterTopologyElement{
					{
						ID: string(TopologyIDCold),
						Size: &models.TopologySize{
							Resource: ec.String("memory"),
							Value:    ec.Int32(memory.ConvertGiBToMiB(64)),
						},
					},
					{
						ID: string(TopologyIDWarm),
						Size: &models.TopologySize{
							Resource: ec.String("memory"),
							Value:    ec.Int32(memory.ConvertGiBToMiB(128)),
						},
					},
					{
						ID: string(TopologyIDHotContent),
						Size: &models.TopologySize{
							Resource: ec.String("memory"),
							Value:    ec.Int32(memory.ConvertGiBToMiB(256)),
						},
					},
				},
			},
			want: &models.ElasticsearchClusterTopologyElement{
				ID: string(TopologyIDHotContent),
				Size: &models.TopologySize{
					Resource: ec.String("memory"),
					Value:    ec.Int32(memory.ConvertGiBToMiB(256)),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FindHotContentTopology(tt.args.topologies); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindHotContentTopology() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalcTopologyNodeNum(t *testing.T) {
	t.Parallel()

	type args struct {
		topology *models.ElasticsearchClusterTopologyElement
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "1 instance x 1 zone",
			args: args{
				topology: &models.ElasticsearchClusterTopologyElement{
					Size: &models.TopologySize{
						Value: ec.Int32(memory.ConvertGiBToMiB(32)),
					},
					ZoneCount: 1,
				},
			},
			want: 1,
		},
		{
			name: "2 instance x 1 zone",
			args: args{
				topology: &models.ElasticsearchClusterTopologyElement{
					Size: &models.TopologySize{
						Value: ec.Int32(memory.ConvertGiBToMiB(128)),
					},
					ZoneCount: 1,
				},
			},
			want: 2,
		},
		{
			name: "3 instance x 3 zone",
			args: args{
				topology: &models.ElasticsearchClusterTopologyElement{
					Size: &models.TopologySize{
						Value: ec.Int32(memory.ConvertGiBToMiB(192)),
					},
					ZoneCount: 3,
				},
			},
			want: 9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcTopologyNodeNum(tt.args.topology); got != tt.want {
				t.Errorf("CalcTopologyNodeNum() = %v, want %v", got, tt.want)
			}
		})
	}
}
