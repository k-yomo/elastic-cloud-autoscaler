package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/k-yomo/elastic-cloud-autoscaler/autoscaler"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	deploymentID := os.Getenv("DEPLOYMENT_ID")
	elasticCloudAPIKey := os.Getenv("ELASTIC_CLOUD_API_KEY")
	elasticsearchCloudID := os.Getenv("ELASTICSEARCH_CLOUD_ID")
	elasticsearchAPIKey := os.Getenv("ELASTICSEARCH_API_KEY")
	monitoringElasticsearchCloudID := os.Getenv("MONITORING_ELASTICSEARCH_CLOUD_ID")
	monitoringElasticsearchAPIKey := os.Getenv("MONITORING_ELASTICSEARCH_API_KEY")

	ecClient, err := api.NewAPI(api.Config{
		Client:     http.DefaultClient,
		AuthWriter: auth.APIKey(elasticCloudAPIKey),
	})
	if err != nil {
		return err
	}

	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		CloudID: elasticsearchCloudID,
		APIKey:  elasticsearchAPIKey,
	})
	if err != nil {
		return err
	}
	monitoringESClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		CloudID: monitoringElasticsearchCloudID,
		APIKey:  monitoringElasticsearchAPIKey,
	})
	if err != nil {
		return err
	}

	esAutoScaler, err := autoscaler.New(&autoscaler.Config{
		ElasticCloudClient:  ecClient,
		DeploymentID:        deploymentID,
		ElasticsearchClient: esClient,
		Scaling: autoscaler.ScalingConfig{
			DefaultMinMemoryGBPerZone: autoscaler.SixtyFourGiBNodeNumToTopologySize(1),
			DefaultMaxMemoryGBPerZone: autoscaler.SixtyFourGiBNodeNumToTopologySize(6),
			AutoScaling: &autoscaler.AutoScalingConfig{
				MetricsProvider:       metrics.NewMonitoringElasticsearchMetricsProvider(monitoringESClient),
				DesiredCPUUtilPercent: 50,
			},
			ScheduledScalings: []*autoscaler.ScheduledScalingConfig{
				{
					StartCronSchedule:  "TZ=UTC 0 0 * * *",
					Duration:           1 * time.Hour,
					MinMemoryGBPerZone: autoscaler.SixtyFourGiBNodeNumToTopologySize(2),
					MaxMemoryGBPerZone: autoscaler.SixtyFourGiBNodeNumToTopologySize(6),
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
