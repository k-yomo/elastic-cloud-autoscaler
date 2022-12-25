package main

import (
	"context"
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/k-yomo/elastic-cloud-autoscaler/autoscaler"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
	"github.com/k0kubun/pp/v3"
	"net/http"
	"os"
	"time"
)

func main() {
	elasticCloudAPIKey := os.Getenv("ELASTIC_CLOUD_API_KEY")
	elasticsearchAPIKey := os.Getenv("ELASTICSEARCH_API_KEY")
	deploymentID := os.Getenv("DEPLOYMENT_ID")
	cloudID := os.Getenv("CLOUD_ID")

	ecClient, err := api.NewAPI(api.Config{
		Client:     http.DefaultClient,
		AuthWriter: auth.APIKey(elasticCloudAPIKey),
	})
	if err != nil {
		panic(err)
	}

	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		APIKey:  elasticsearchAPIKey,
		CloudID: cloudID,
	})
	if err != nil {
		panic(err)
	}

	esAutoscaler, err := autoscaler.New(&autoscaler.Config{
		ElasticCloudClient:  ecClient,
		DeploymentID:        deploymentID,
		ElasticsearchClient: esClient,
		Scaling: autoscaler.ScalingConfig{
			DefaultMinSizeMemoryGB: 1,
			DefaultMaxSizeMemoryGB: 2,
			AutoScaling: &autoscaler.AutoScalingConfig{
				MetricsProvider:       metrics.NewMonitoringElasticsearchMetricsProvider(esClient),
				DesiredCPUUtilPercent: 50,
			},
			ScheduledScalings: []*autoscaler.ScheduledScalingConfig{
				{
					StartCronSchedule: "TZ=UTC 29 14 * * *",
					Duration:          1 * time.Hour,
					MinSizeMemoryGB:   2,
					MaxSizeMemoryGB:   2,
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	scalingOperation, err := esAutoscaler.Run(context.Background())
	if err != nil {
		panic(err)
	}
	pp.Println(scalingOperation)
}
