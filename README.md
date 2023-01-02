# elastic-cloud-autoscaler

![License: Apache-2.0](https://img.shields.io/badge/License-MIT-blue.svg)
[![Test](https://github.com/k-yomo/elastic-cloud-autoscaler/actions/workflows/test.yml/badge.svg)](https://github.com/k-yomo/elastic-cloud-autoscaler/actions/workflows/test.yml)
[![Codecov](https://codecov.io/gh/k-yomo/elastic-cloud-autoscaler/branch/main/graph/badge.svg?token=P3pNbMGbeN)](https://codecov.io/gh/k-yomo/elastic-cloud-autoscaler)
[![Go Report Card](https://goreportcard.com/badge/k-yomo/elastic-cloud-autoscaler)](https://goreportcard.com/report/k-yomo/elastic-cloud-autoscaler)

Elastic Cloud Autoscaler based on CPU util or cron schedules.

**⚠️ This library is still experimental, please use at your own risk if you use.**
I also highly recommend using with `DryRun: true` at first.

## Compatibility
- Elasticsearch >= 8.x

## Constraints
- Monitoring deployment must be enabled to use this library.
  https://www.elastic.co/guide/en/cloud/current/ec-enable-logging-and-monitoring.html#ec-enable-logging-and-monitoring-steps
- This library only support topology size greater than or equal to `64g` for now.

## Example
```go
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
			DefaultMinMemoryGBPerZone: autoscaler.SixtyFourGiBNodeNumToTopologySize(1),
			DefaultMaxMemoryGBPerZone: autoscaler.SixtyFourGiBNodeNumToTopologySize(6),
			AutoScaling: &autoscaler.AutoScalingConfig{
				MetricsProvider:       metrics.NewMonitoringElasticsearchMetricsProvider(monitoringESClient),
				DesiredCPUUtilPercent: 50,
			},
			ScheduledScalings: []*autoscaler.ScheduledScalingConfig{
				{
					StartCronSchedule: "TZ=UTC 0 0 * * *",
					Duration:          1 * time.Hour,
					MinMemoryGBPerZone:   autoscaler.SixtyFourGiBNodeNumToTopologySize(2),
					MaxMemoryGBPerZone:   autoscaler.SixtyFourGiBNodeNumToTopologySize(6),
				},
			},
			Index: "test-index",
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
```

#### Demo
You can easily test this library with the below repository.
https://github.com/k-yomo/elastic-cloud-autoscaler-demo
