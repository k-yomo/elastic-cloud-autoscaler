package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/elasticsearch"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/ptrutil"
	"strconv"
	"time"
)

// monitoringElasticsearchMetricsProvider is a metrics provider to fetch metrics from elasticsearch on monitoring deployment.
// Monitoring deployment must be configured with the below way.
// https://www.elastic.co/guide/en/cloud/current/ec-enable-logging-and-monitoring.html#ec-enable-logging-and-monitoring-steps
type monitoringElasticsearchMetricsProvider struct {
	esClient *esv8.TypedClient
}

func NewMonitoringElasticsearchMetricsProvider(monitoringESClient *esv8.TypedClient) Provider {
	return &monitoringElasticsearchMetricsProvider{
		esClient: monitoringESClient,
	}
}

func (m *monitoringElasticsearchMetricsProvider) GetCPUUtilMetrics(ctx context.Context, targetNodeNames []string, after time.Time) (AvgCPUUtils, error) {
	afterUnixMilliStr := strconv.FormatInt(after.Unix(), 10)
	searchRequest := search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Filter: []types.Query{
					{
						Range: map[string]types.RangeQuery{
							"@timestamp": types.DateRangeQuery{
								Gt: &afterUnixMilliStr,
							},
						},
					},
					{
						Term: map[string]types.TermQuery{
							"metricset.name": {
								Value: "node_stats",
							},
						},
					},
					{
						Terms: &types.TermsQuery{
							TermsQuery: map[string]types.TermsQueryField{
								"elasticsearch.node.name": targetNodeNames,
							},
						},
					},
				},
			},
		},
		Aggregations: map[string]types.Aggregations{
			"histogram": {
				DateHistogram: &types.DateHistogramAggregation{
					Field:         ptrutil.ToPtr("@timestamp"),
					FixedInterval: ptrutil.ToPtr[types.Duration]("1m"),
				},
				Aggregations: map[string]types.Aggregations{
					"avg_cpu_util": {
						Avg: &types.AverageAggregation{
							Field: ptrutil.ToPtr("elasticsearch.node.stats.process.cpu.pct"),
						},
					},
				},
			},
		},
		TrackTotalHits: ptrutil.ToPtr[types.TrackHits](false),
		Source_:        ptrutil.ToPtr[types.SourceConfig](false),
	}
	searchRequestJSON, err := json.Marshal(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal search input to json: %w", err)
	}
	resp, err := m.esClient.Search().Raw(searchRequestJSON).Index(".ds-.monitoring-es-8-mb-*").Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("search api call: %w", err)
	}
	if err := elasticsearch.ExtractError(resp); err != nil {
		return nil, err
	}

	var result GetCPUUtilMetricsQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unmarshal search result: %w", err)
	}

	cpuUtils := make([]*AvgCPUUtil, 0, len(result.Aggregations.Histogram.Buckets))
	for _, bucket := range result.Aggregations.Histogram.Buckets {
		cpuUtils = append(cpuUtils, &AvgCPUUtil{
			Timestamp: time.UnixMilli(bucket.Key),
			Percent:   bucket.AvgCPUUtil.Value,
		})
	}

	return cpuUtils, nil
}

type GetCPUUtilMetricsQueryResult struct {
	Aggregations struct {
		Histogram struct {
			Buckets []struct {
				Key        int64 `json:"key"`
				AvgCPUUtil struct {
					Value float64 `json:"value"`
				} `json:"avg_cpu_util"`
			} `json:"buckets"`
		} `json:"Histogram"`
	} `json:"aggregations"`
}
