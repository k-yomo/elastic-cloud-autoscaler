package autoscaler

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/go-openapi/strfmt"

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
)

func TestNew(t *testing.T) {
	t.Parallel()

	validConfig := &Config{
		DeploymentID:        "test",
		ElasticCloudClient:  &api.API{},
		ElasticsearchClient: &esv8.TypedClient{},
		Scaling: ScalingConfig{
			DefaultMinMemoryGBPerZone: 128,
			DefaultMaxMemoryGBPerZone: 256,

			AutoScaling: &AutoScalingConfig{
				MetricsProvider:       &mock_metrics.MockProvider{},
				DesiredCPUUtilPercent: 50,
			},
			ScheduledScalings: []*ScheduledScalingConfig{
				{
					StartCronSchedule:  "TZ=UTC 29 14 * * *",
					Duration:           1 * time.Hour,
					MinMemoryGBPerZone: 192,
					MaxMemoryGBPerZone: 256,
				},
			},
			Index:         "test-index",
			ShardsPerNode: 1,
		},
	}

	type args struct {
		config *Config
	}
	tests := []struct {
		name    string
		args    args
		want    *AutoScaler
		wantErr bool
	}{
		{
			name: "valid config",
			args: args{
				config: validConfig,
			},
			want: &AutoScaler{
				config:   validConfig,
				ecClient: elasticcloud.NewClient(validConfig.ElasticCloudClient, validConfig.DeploymentID),
				esClient: elasticsearch.NewClient(validConfig.ElasticsearchClient),
			},
		},
		{
			name: "invalid config",
			args: args{
				config: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoScaler_Run(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		prepareMocks func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider)
		want         *ScalingOperation
		wantErr      bool
	}{
		{
			name: "scaling out",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             1,
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
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   1,
					ReplicaNum: 1,
				}, nil)

				ecClient.EXPECT().
					UpdateESHotContentTopologySize(gomock.Any(), elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(2))).
					Return(nil)

				esClient.EXPECT().
					UpdateIndexReplicaNum(gomock.Any(), "test-index", 3).
					Return(nil)
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
			name: "scaling in",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
					Index:                     "test-index",
					ShardsPerNode:             1,
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(SixtyFourGiBNodeNumToTopologySize(2), 2),
						},
					},
				}, nil)
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   1,
					ReplicaNum: 3,
				}, nil)

				esClient.EXPECT().
					UpdateIndexReplicaNum(gomock.Any(), "test-index", 1).
					Return(nil)

				ecClient.EXPECT().
					UpdateESHotContentTopologySize(gomock.Any(), elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1))).
					Return(nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(2)),
				ToTopologySize:   elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1)),
				FromReplicaNum:   3,
				ToReplicaNum:     1,
				Reason:           "current or desired topology size '128g' is greater than max topology size '64g'",
			},
		},
		{
			name: "scaling operation doesn't run when DryRun = true",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             1,
				},
				DryRun: true,
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(64, 2),
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

			a := &AutoScaler{
				config:   tt.config,
				ecClient: mockECClient,
				esClient: mockESClient,
			}
			got, err := a.Run(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Run() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoScaler_CalcScalingOperation(t *testing.T) {
	now := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
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
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             1,
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
			name: "scaling out - cpu utilization exceeds threshold",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(3)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(6)),
					Index:                     "test-index",
					ShardsPerNode:             1,
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
				Reason:           "CPU utilization (currently '80.0%') is higher than the desired CPU utilization '50%' for 600 seconds",
			},
		},
		{
			name: "scaling out - current size < scheduled scaling min size",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             1,
					ScheduledScalings: []*ScheduledScalingConfig{
						{
							MinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
							MaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
							StartCronSchedule:  "TZ=UTC 0 0 * * *",
							Duration:           time.Minute,
						},
					},
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
			name: "scaling in - current size > scheduled scaling max size",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             1,
					ScheduledScalings: []*ScheduledScalingConfig{
						{
							MinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
							MaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
							StartCronSchedule:  "TZ=UTC 0 0 * * *",
							Duration:           time.Minute,
						},
					},
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(128, 2),
						},
					},
				}, nil)
				esClient.EXPECT().GetIndexSettings(gomock.Any(), "test-index").Return(&elasticsearch.IndexSettings{
					ShardNum:   1,
					ReplicaNum: 1,
				}, nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(2)),
				ToTopologySize:   elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1)),
				FromReplicaNum:   1,
				ToReplicaNum:     1,
				Reason:           "current or desired topology size '128g' is greater than max topology size '64g'",
			},
		},
		{
			name: "scaling in - cpu utilization is lower than threshold",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(6)),
					Index:                     "test-index",
					ShardsPerNode:             1,
					AutoScaling: &AutoScalingConfig{
						DesiredCPUUtilPercent:     50,
						ScaleOutThresholdDuration: 10 * time.Minute,
						ScaleInThresholdDuration:  10 * time.Minute,
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
						"node-0001": newNodeStatsNode("node-0001", 20),
						"node-0002": newNodeStatsNode("node-0002", 20),
						"node-0003": newNodeStatsNode("node-0003", 20),
						"node-0004": newNodeStatsNode("node-0004", 20),
						"node-0005": newNodeStatsNode("node-0005", 20),
						"node-0006": newNodeStatsNode("node-0006", 20),
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
					{Percent: 20, Timestamp: now},
				}, nil)
			},
			want: &ScalingOperation{
				FromTopologySize: elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(3)),
				ToTopologySize:   elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1)),
				FromReplicaNum:   1,
				ToReplicaNum:     1,
				Reason:           "CPU utilization (currently '20.0%') is lower than the desired CPU utilization '50%' for 600 seconds",
			},
		},
		{
			name: "scaling is not eligible when cool down period",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(1)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(4)),
					Index:                     "test-index",
					ShardsPerNode:             1,
					AutoScaling: &AutoScalingConfig{
						DesiredCPUUtilPercent:     50,
						ScaleOutThresholdDuration: 10 * time.Minute,
						ScaleOutCoolDownDuration:  10 * time.Minute,
					},
				},
			},
			prepareMocks: func(ecClient *mock_elasticcloud.MockClient, esClient *mock_elasticsearch.MockClient, metricsProvider *mock_metrics.MockProvider) {
				ecClient.EXPECT().GetESResourceInfo(gomock.Any(), true).Return(&models.ElasticsearchResourceInfo{
					Info: &models.ElasticsearchClusterInfo{
						PlanInfo: &models.ElasticsearchClusterPlansInfo{
							Current: newElasticsearchClusterPlanInfo(SixtyFourGiBNodeNumToTopologySize(2), 2),
							History: []*models.ElasticsearchClusterPlanInfo{
								{
									Plan: &models.ElasticsearchClusterPlan{
										ClusterTopology: []*models.ElasticsearchClusterTopologyElement{
											{
												ID:        string(elasticcloud.TopologyIDHotContent),
												Size:      elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1)),
												ZoneCount: 2,
											},
										},
									},
									AttemptEndTime: func() strfmt.DateTime {
										dt, _ := strfmt.ParseDateTime(now.Add(-1 * time.Hour).Format(time.RFC3339))
										return dt
									}(),
								},
								{
									Plan: &models.ElasticsearchClusterPlan{
										ClusterTopology: []*models.ElasticsearchClusterTopologyElement{
											{
												ID:        string(elasticcloud.TopologyIDHotContent),
												Size:      elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(1)),
												ZoneCount: 2,
											},
										},
									},
									AttemptEndTime: func() strfmt.DateTime {
										dt, _ := strfmt.ParseDateTime(now.Format(time.RFC3339))
										return dt
									}(),
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
				FromTopologySize: elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(2)),
				ToTopologySize:   elasticcloud.NewTopologySize(SixtyFourGiBNodeNumToTopologySize(2)),
				FromReplicaNum:   1,
				ToReplicaNum:     1,
				Reason:           "Currently within cool down period",
			},
		},
		{
			name: "scaling is not eligible when pending plan exists",
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             1,
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
			name: "scaling is not eligible when no available topology size",
			// min/max node: 2, zone count: 2 => 4 nodes
			// shard num: 3, shards per node: 4
			// To satisfy the requirement, 16 shards will be needed, but 3x shards can't be 16
			config: &Config{
				DeploymentID: "test",
				Scaling: ScalingConfig{
					DefaultMinMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					DefaultMaxMemoryGBPerZone: int(SixtyFourGiBNodeNumToTopologySize(2)),
					Index:                     "test-index",
					ShardsPerNode:             4,
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

			a := &AutoScaler{
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
