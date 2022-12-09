package autoscaler

import (
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/go-elasticsearch/v8"
	mock_metrics "github.com/k-yomo/elastic-cloud-autoscaler/mocks/metrics"
	"testing"
	"time"
)

func Test_validateConfig(t *testing.T) {
	type args struct {
		config *Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid config",
			args: args{
				config: &Config{
					DeploymentID:        "test",
					ElasticCloudClient:  &api.API{},
					ElasticsearchClient: &elasticsearch.TypedClient{},
					Scaling: ScalingConfig{
						DefaultMinSizeMemoryGB: 128,
						DefaultMaxSizeMemoryGB: 256,

						AutoScaling: &AutoScalingConfig{
							MetricsProvider:       &mock_metrics.MockProvider{},
							DesiredCPUUtilPercent: 50,
						},
						ScheduledScalings: []*ScheduledScalingConfig{
							{
								StartCronSchedule: "TZ=UTC 29 14 * * *",
								Duration:          1 * time.Hour,
								MinSizeMemoryGB:   192,
								MaxSizeMemoryGB:   256,
							},
						},
						Index:         "test-index",
						ShardsPerNode: 1,
					},
				},
			},
		},
		{
			name: "cron schedule is invalid",
			args: args{
				config: &Config{
					DeploymentID:        "test",
					ElasticCloudClient:  &api.API{},
					ElasticsearchClient: &elasticsearch.TypedClient{},
					Scaling: ScalingConfig{
						DefaultMinSizeMemoryGB: 128,
						DefaultMaxSizeMemoryGB: 256,

						AutoScaling: &AutoScalingConfig{
							MetricsProvider:       &mock_metrics.MockProvider{},
							DesiredCPUUtilPercent: 50,
						},
						ScheduledScalings: []*ScheduledScalingConfig{
							{
								StartCronSchedule: "invalid",
								Duration:          1 * time.Hour,
								MinSizeMemoryGB:   192,
								MaxSizeMemoryGB:   256,
							},
						},
						Index:         "test-index",
						ShardsPerNode: 1,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateConfig(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
