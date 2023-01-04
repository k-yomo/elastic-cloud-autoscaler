package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/k-yomo/elastic-cloud-autoscaler/autoscaler"
	"github.com/k-yomo/elastic-cloud-autoscaler/metrics"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("initialize zap: %w", err)
	}
	defer logger.Sync()

	configFilePath := os.Getenv("CONFIG_FILE_PATH")
	deploymentID := mustEnv("DEPLOYMENT_ID")
	elasticCloudAPIKey := mustEnv("ELASTIC_CLOUD_API_KEY")
	elasticsearchCloudID := mustEnv("ELASTICSEARCH_CLOUD_ID")
	elasticsearchAPIKey := mustEnv("ELASTICSEARCH_API_KEY")
	monitoringElasticsearchCloudID := mustEnv("MONITORING_ELASTICSEARCH_CLOUD_ID")
	monitoringElasticsearchAPIKey := mustEnv("MONITORING_ELASTICSEARCH_API_KEY")
	var dryRun bool
	if dryRunStr := os.Getenv("DRY_RUN"); dryRunStr != "" {
		dryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			return fmt.Errorf("parse DRY_RUN env var: %w", err)
		}
	}

	configBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	scalingConfig := autoscaler.ScalingConfig{}
	if err := yaml.Unmarshal(configBytes, &scalingConfig); err != nil {
		return fmt.Errorf("unmarhsal config: %w", err)
	}

	ecClient, err := api.NewAPI(api.Config{
		Client:     http.DefaultClient,
		AuthWriter: auth.APIKey(elasticCloudAPIKey),
	})
	if err != nil {
		return fmt.Errorf("initialize elastic cloud API client: %w", err)
	}

	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		APIKey:  elasticsearchAPIKey,
		CloudID: elasticsearchCloudID,
	})
	if err != nil {
		return fmt.Errorf("initialize elasticsearch API client: %w", err)
	}

	if scalingConfig.AutoScaling != nil {
		monitoringESClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
			APIKey:  monitoringElasticsearchAPIKey,
			CloudID: monitoringElasticsearchCloudID,
		})
		if err != nil {
			return fmt.Errorf("initialize monitoring elasticsearch API client: %w", err)
		}
		scalingConfig.AutoScaling.MetricsProvider = metrics.NewMonitoringElasticsearchMetricsProvider(monitoringESClient)
	}

	esAutoscaler, err := autoscaler.New(&autoscaler.Config{
		ElasticCloudClient:  ecClient,
		DeploymentID:        deploymentID,
		ElasticsearchClient: esClient,
		Scaling:             scalingConfig,
		DryRun:              dryRun,
	})
	if err != nil {
		return fmt.Errorf("initialize elastic cloud autoscaler: %w", err)
	}
	scalingOperation, err := esAutoscaler.Run(context.Background())
	if err != nil {
		return fmt.Errorf("run elastic cloud autoscaler: %w", err)
	}
	if scalingOperation.Direction() != autoscaler.ScalingDirectionNone || scalingOperation.FromReplicaNum != scalingOperation.ToReplicaNum {
		logger.Info("scaling operation is applied",
			zap.String("scalingDirection", string(scalingOperation.Direction())),
			zap.Int32p("fromTopologySize", scalingOperation.FromTopologySize.Value),
			zap.Int32p("toTopologySize", scalingOperation.ToTopologySize.Value),
			zap.Int("fromReplicaNum", scalingOperation.FromReplicaNum),
			zap.Int("toReplicaNum", scalingOperation.ToReplicaNum),
			zap.String("reason", scalingOperation.Reason),
		)
	} else {
		logger.Info("scaling operation is not eligible", zap.String("reason", scalingOperation.Reason))
	}

	return nil
}

func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		log.Panicf("env variable '%s' must be set", key)
	}
	return v
}
