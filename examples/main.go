package main

import (
	"context"
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/k-yomo/elastic-cloud-autoscaler/autoscaler"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	deploymentID := os.Getenv("DEPLOYMENT_ID")
	elasticCloudAPIKey := os.Getenv("ELASTIC_CLOUD_API_KEY")
	elasticsearchAPIKey := os.Getenv("ELASTICSEARCH_API_KEY")
	monitoringElasticsearchAPIKey := os.Getenv("MONITORING_ELASTICSEARCH_API_KEY")
	elasticsearchCloudID := os.Getenv("ELASTICSEARCH_CLOUD_ID")
	monitoringElasticsearchCloudID := os.Getenv("MONITORING_ELASTICSEARCH_CLOUD_ID")

	ecClient, err := api.NewAPI(api.Config{
		Client:     http.DefaultClient,
		AuthWriter: auth.APIKey(elasticCloudAPIKey),
	})
	if err != nil {
		return err
	}

	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		APIKey:  elasticsearchAPIKey,
		CloudID: elasticsearchCloudID,
	})
	if err != nil {
		return err
	}
	monitoringESClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		APIKey:  monitoringElasticsearchAPIKey,
		CloudID: monitoringElasticsearchCloudID,
	})
	if err != nil {
		return err
	}

	esAutoScaler, err := autoscaler.New(&autoscaler.Config{
		ElasticCloudClient:  ecClient,
		DeploymentID:        deploymentID,
		ElasticsearchClient: esClient,
		Scaling: autoscaler.ScalingConfig{
			DefaultMinSizeMemoryGB: 64,
			DefaultMaxSizeMemoryGB: 382,
			AutoScaling: &autoscaler.AutoScalingConfig{
				MetricsProvider:       metrics.NewMonitoringElasticsearchMetricsProvider(monitoringESClient),
				DesiredCPUUtilPercent: 50,
			},
			ScheduledScalings: []*autoscaler.ScheduledScalingConfig{
				{
					StartCronSchedule: "TZ=UTC 0 0 * * *",
					Duration:          1 * time.Hour,
					MinSizeMemoryGB:   128,
					MaxSizeMemoryGB:   384,
				},
			},
			Index:         "test-index",
			ShardsPerNode: 4,
		},
	})
	if err != nil {
		return err
	}
	for {
		select {
		case <-time.After(1 * time.Minute):
			scalingOperation, err := esAutoScaler.Run(context.Background())
			if err != nil {
				return err
			}
			if scalingOperation.Direction() != autoscaler.ScalingDirectionNone {
				fmt.Println("scaling direction", scalingOperation.Direction())
				fmt.Println("updated topology size", *scalingOperation.ToTopologySize.Value)
				fmt.Println("reason", scalingOperation.Reason)
			}
		}
	}
}
