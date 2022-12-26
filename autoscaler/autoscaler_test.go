package autoscaler

import (
	"context"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
	mock_metrics "github.com/k-yomo/elastic-cloud-autoscaler/mocks/metrics"
	mock_elasticcloud "github.com/k-yomo/elastic-cloud-autoscaler/mocks/pkg/elasticcloud"
	mock_elasticsearch "github.com/k-yomo/elastic-cloud-autoscaler/mocks/pkg/elasticsearch"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/clock"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticcloud"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticsearch"
	"testing"
	"time"
)

func TestAutoScalar_CalcScalingOperation(t *testing.T) {
	now := time.Now()
	resetNow := clock.MockTime(t, now)
	defer resetNow()

	tests := []struct {
		name         string
		config       *Config
		prepareMocks func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider)
		want         *ScalingOperation
		wantErr      bool
	}{
		{
			name: "scaling out - current size < default min size",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                  "test-index",
					ShardsPerNode:          1,
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(64, 2),
						},
					},
				}, nil)
				esClient.EXPECT().GetNodeStats(gomock.Any()).Return(&elasticsearch.NodeStats{
					Nodes: map[string]*elasticsearch.NodeStatsNode{
						"node-0001": {
							ID:    "node-0001",
							Roles: []string{"data_content"},
							OS: &elasticsearch.NodeStatsNodeOS{
								CPU: &elasticsearch.NodeStatsNodeOSCPU{
									Percent: 50,
								},
							},
						},
					},
				}, nil)
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   1,
					ReplicaNum: 1,
				}, nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1)),
				ToTopologySize:   elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(2)),
				FromReplicaNum:   1,
				ToReplicaNum:     3,
				Reason:           "current or desired topology size '64g' is less than min topology size '128g'",
			},
		},
		{
			name: "scaling out - cpu exceeds threshold",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(3)),
					DefaultMaxSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(6)),
					Index:                  "test-index",
					ShardsPerNode:          1,
					AutoScaling: &AutoScalingConfig{
						DesiredCPUUtilPercent:     50,
						ScaleOutThresholdDuration: 10 * time.Minute,
						ScaleOutCoolDownDuration:  0,
					},
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(SixtyFourGiBNodeNumToTopologySize(3), 2),
						},
					},
				}, nil)
				esClient.EXPECT().GetNodeStats(gomock.Any()).Return(&elasticsearch.NodeStats{
					Nodes: map[string]*elasticsearch.NodeStatsNode{
						"node-0001": newNodeStatsNode("node-0001", 80),
						"node-0002": newNodeStatsNode("node-0002", 80),
						"node-0003": newNodeStatsNode("node-0003", 80),
						"node-0004": newNodeStatsNode("node-0004", 80),
						"node-0005": newNodeStatsNode("node-0005", 80),
						"node-0006": newNodeStatsNode("node-0006", 80),
					},
				}, nil)
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   1,
					ReplicaNum: 1,
				}, nil)
				metricsProvider.EXPECT().GetCPUUtilMetrics(
					gomock.Any(),
					[]string{"node-0001", "node-0002", "node-0003", "node-0004", "node-0005", "node-0006"},
					now.Add(-10*time.Minute),
				).Return(metrics.AvgCPUUtils{
					{Percent: 80, Timestamp: now},
				}, nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(3)),
				ToTopologySize:   elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(5)),
				FromReplicaNum:   1,
				ToReplicaNum:     9,
				Reason:           "CPU utilization is greater than the desired CPU utilization '50%' for 600 seconds",
			},
		},
		{
			name: "scaling is not available when pending plan exists",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                  "test-index",
					ShardsPerNode:          1,
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(64, 2),
							Pending: newElasticsearchClusterPlanInfo(64, 2),
						},
					},
				}, nil)
				esClient.EXPECT().GetNodeStats(gomock.Any()).Return(&elasticsearch.NodeStats{
					Nodes: map[string]*elasticsearch.NodeStatsNode{},
				}, nil)
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   1,
					ReplicaNum: 1,
				}, nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(64),
				ToTopologySize:   elasticcloud.NewTopologySize(64),
				FromReplicaNum:   1,
				ToReplicaNum:     1,
				Reason:           "Pending plan exists",
			},
		},
		{
			name: "scaling is not available when no available topology size",
			// min/max node: 2, zone count: 2 => 4 nodes
			// shard num: 3, shards per node: 4
			// To satisfy the requirement, 16 shards will be needed, but 3x shards can't be 16
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxSizeMemoryGB: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                  "test-index",
					ShardsPerNode:          4,
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(64, 2),
						},
					},
				}, nil)
				esClient.EXPECT().GetNodeStats(gomock.Any()).Return(&elasticsearch.NodeStats{
					Nodes: map[string]*elasticsearch.NodeStatsNode{},
				}, nil)
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   3,
					ReplicaNum: 1,
				}, nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(64),
				ToTopologySize:   elasticcloud.NewTopologySize(64),
				FromReplicaNum:   1,
				ToReplicaNum:     1,
				Reason:           "No possible topology size with the given configuration",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockECClient := mock_elasticcloud.NewMockClient(ctrl)
			mockESClient := mock_elasticsearch.NewMockClient(ctrl)
			mockMetricsProvider := mock_metrics.NewMockProvider(ctrl)

			if tt.prepareMocks != nil {
				tt.prepareMocks(mockECClient, mockESClient, mockMetricsProvider)
			}

			if tt.config.Scaling.AutoScaling != nil {
				tt.config.Scaling.AutoScaling.MetricsProvider = mockMetricsProvider
			}

			a := &AutoScalar{
				config:   tt.config,
				ecClient: mockECClient,
				esClient: mockESClient,
			}
			got, err := a.CalcScalingOperation(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("CalcScalingOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("CalcScalingOperation() diff(-want +got):\n%s", diff)
			}
		})
	}
}

func newNodeStatsNode(id string, cpuUtil float64) *elasticsearch.NodeStatsNode {
	return &elasticsearch.NodeStatsNode{
		ID:    id,
		Roles: []string{"data_content"},
		OS: &elasticsearch.NodeStatsNodeOS{
			CPU: &elasticsearch.NodeStatsNodeOSCPU{
				Percent: cpuUtil,
			},
		},
	}
}

func newElasticsearchClusterPlanInfo(topologySizeGiB int32, zoneCount int32) *models.ElasticsearchClusterPlanInfo {
	return &models.ElasticsearchClusterPlanInfo{
		Plan: &models.ElasticsearchClusterPlan{
			ClusterTopology: []*models.ElasticsearchClusterTopologyElement{
				{
					ID:        string(elasticcloud.TopologyIDHotContent),
					Size:      elasticcloud.NewTopologySize(topologySizeGiB),
					ZoneCount: zoneCount,
				},
			},
		},
	}
}
