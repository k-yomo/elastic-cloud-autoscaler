package main

import (
	"context"
	"encoding/json"
	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/k0kubun/pp/v3"
	"net/http"
)

func main() {
	elasticCloudAPIKey := "SDRPWDhvUUJBVVV6UlROa0RPUUM6U1k4UVJNNlNUZC1LV3pjTEhscWxlZw=="
	elasticsearchAPIKey := "UkFVaTg0UUJLNnk1Tk90QVJnX3Y6UTJMeTdHVVRSQXF5cmZrUjQ2ZFJzdw=="
	deploymentID := "1ea094e2c8f24988adc4db7b5f839423"

	authWriter, err := auth.NewAuthWriter(auth.Config{
		APIKey: elasticCloudAPIKey,
	})
	if err != nil {
		panic(err)
	}
	ecAPI, err := api.NewAPI(api.Config{
		Client:     http.DefaultClient,
		AuthWriter: authWriter,
	})
	if err != nil {
		panic(err)
	}

	ecClient, err := NewClient(ecAPI, deploymentID)
	if err != nil {
		panic(err)
	}
	_ = ecClient
	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		CloudID: ecClient.cloudID,
		APIKey:  elasticsearchAPIKey,
	})
	if err != nil {
		panic(err)
	}
	if err := FetchNodeStats(esClient); err != nil {
		panic(err)
	}

	//size, err := deploymentsize.ParseGb("1gb")
	//if err != nil {
	//	panic(err)
	//}
	//topologyIDSizeMap := map[string]*models.TopologySize{
	//	"hot_content": {
	//		Resource: ec.String("memory"),
	//		Value:    ec.Int32(size),
	//	},
	//}
	//if err := ecClient.UpdateElasticsearchTopologySize(topologyIDSizeMap); err != nil {
	//	panic(err)
	//}
}

func FetchNodeStats(esClient *elasticsearch.TypedClient) error {
	resp, err := esClient.Nodes.Stats().Metric("os").Do(context.Background())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var nodeStats NodeStats
	if err := json.NewDecoder(resp.Body).Decode(&nodeStats); err != nil {
		return err
	}
	pp.Println(nodeStats)
	return nil
}

type NodeStats struct {
	Nodes map[string]struct {
		OS struct {
			CPU struct {
				LoadAverage struct {
					OneMinute     float64 `json:"1m"`
					FiveMinute    float64 `json:"5m"`
					FifteenMinute float64 `json:"15m"`
				} `json:"load_average"`
				Percent float64 `json:"percent"`
			} `json:"cpu"`
		} `json:"os"`
		Roles []string `json:"roles"`
	}
}
