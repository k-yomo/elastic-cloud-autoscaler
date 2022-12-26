package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/util/slice"
	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"strconv"
)

//go:generate mockgen -source=$GOFILE -package=mock_$GOPACKAGE -destination=../../mocks/pkg/$GOPACKAGE/mock_$GOFILE
type Client interface {
	GetNodeStats(ctx context.Context) (*NodeStats, error)
	GetIndexSettings(ctx context.Context, indexName string) (*IndexSettings, error)
	GetIndexHealth(ctx context.Context, indexName string) (*IndexHealth, error)
	UpdateIndexReplicaNum(ctx context.Context, indexName string, replicaNum int) error
}

type clientImpl struct {
	esClient *esv8.TypedClient
}

func NewClient(esClient *esv8.TypedClient) Client {
	return &clientImpl{esClient: esClient}
}

type NodeStats struct {
	Nodes map[string]*NodeStatsNode `json:"nodes"`
}

func (n *NodeStats) DataContentNodes() NodeStatsNodes {
	var dataContentNodes []*NodeStatsNode
	for _, node := range n.Nodes {
		if slice.HasString(node.Roles, "data_content") {
			dataContentNodes = append(dataContentNodes, node)
		}
	}
	return dataContentNodes
}

type NodeStatsNode struct {
	ID    string   `json:"-"`
	Roles []string `json:"roles"`
	OS    struct {
		CPU struct {
			Percent float64 `json:"percent"`
		} `json:"cpu"`
	} `json:"os"`
}

type NodeStatsNodes []*NodeStatsNode

func (n NodeStatsNodes) IDs() []string {
	ids := make([]string, 0, len(n))
	for _, node := range n {
		ids = append(ids, node.ID)
	}
	return ids
}

func (n NodeStatsNodes) TotalCPUUtil() float64 {
	totalCPUUtil := 0.0
	for _, node := range n {
		totalCPUUtil += node.OS.CPU.Percent
	}
	return totalCPUUtil
}

func (n NodeStatsNodes) AvgCPUUtil() float64 {
	return n.TotalCPUUtil() / float64(len(n))
}

func (c *clientImpl) GetNodeStats(ctx context.Context) (*NodeStats, error) {
	resp, err := c.esClient.Nodes.Stats().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("node stats api call: %w", err)
	}
	defer resp.Body.Close()

	if err := ExtractError(resp); err != nil {
		return nil, fmt.Errorf("get node stats: %w", err)
	}

	var nodeStats NodeStats
	if err := json.NewDecoder(resp.Body).Decode(&nodeStats); err != nil {
		return nil, fmt.Errorf("decode node stats api response: %w", err)
	}
	for id, node := range nodeStats.Nodes {
		node.ID = id
	}
	return &nodeStats, nil
}

type IndexSettings struct {
	ShardNum   int `json:"number_of_shards,string"`
	ReplicaNum int `json:"number_of_replicas,string"`
}

// TotalShardNum returns total shard count of primary + replica shards
func (i *IndexSettings) TotalShardNum() int {
	return CalcTotalShardNum(i.ShardNum, i.ReplicaNum)
}

func (c *clientImpl) GetIndexSettings(ctx context.Context, indexName string) (*IndexSettings, error) {
	resp, err := c.esClient.Indices.GetSettings().Index(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("index settings api call: %w", err)
	}
	defer resp.Body.Close()

	if err := ExtractError(resp); err != nil {
		return nil, fmt.Errorf("get index '%s' settings: %w", indexName, err)
	}

	var indexSettings map[string]struct {
		Index struct {
			Settings *IndexSettings `json:"settings"`
		} `json:"index"`
		IndexSettings
	}
	if err := json.NewDecoder(resp.Body).Decode(&indexSettings); err != nil {
		return nil, fmt.Errorf("decode index settings api response: %w", err)
	}

	index, ok := indexSettings[indexName]
	if !ok || len(indexSettings) > 1 {
		return nil, errors.New("index name must not be alias")
	}

	return index.Index.Settings, nil
}

type IndexHealth struct {
	ClusterName                 string `json:"cluster_name"`
	Status                      string `json:"status"`
	NumberOfNodes               int    `json:"number_of_nodes"`
	NumberOfDataNodes           int    `json:"number_of_data_nodes"`
	ActivePrimaryShards         int    `json:"active_primary_shards"`
	ActiveShards                int    `json:"active_shards"`
	RelocatingShards            int    `json:"relocating_shards"`
	InitializingShards          int    `json:"initializing_shards"`
	UnassignedShards            int    `json:"unassigned_shards"`
	DelayedUnassignedShards     int    `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks        int    `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch       int    `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis int    `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber int    `json:"active_shards_percent_as_number"`
}

func (i *IndexHealth) IsHealthy() bool {
	return i.Status == "green" &&
		i.RelocatingShards == 0 &&
		i.InitializingShards == 0 &&
		i.UnassignedShards == 0 &&
		i.DelayedUnassignedShards == 0
}

func (c *clientImpl) GetIndexHealth(ctx context.Context, indexName string) (*IndexHealth, error) {
	resp, err := c.esClient.Cluster.Health().Index(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("index health api call: %w", err)
	}
	defer resp.Body.Close()

	if err := ExtractError(resp); err != nil {
		return nil, fmt.Errorf("get index '%s' health: %w", indexName, err)
	}

	var indexHealth IndexHealth
	if err := json.NewDecoder(resp.Body).Decode(&indexHealth); err != nil {
		return nil, fmt.Errorf("decode index health api response: %w", err)
	}

	return &indexHealth, nil
}

func (c *clientImpl) UpdateIndexReplicaNum(ctx context.Context, indexName string, replicaNum int) error {
	settings := types.IndexSettings{
		NumberOfReplicas: strconv.Itoa(replicaNum),
	}
	settingsJSON, err := settings.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal settings json: %w", err)
	}
	resp, err := c.esClient.Indices.PutSettings().Index(indexName).Raw(settingsJSON).Do(ctx)
	if err != nil {
		return fmt.Errorf("put settings: %w", err)
	}
	defer resp.Body.Close()

	if err := ExtractError(resp); err != nil {
		return fmt.Errorf("update number_of_replica: %w", err)
	}

	// TODO: wait until shard relocation finishs

	return nil
}
