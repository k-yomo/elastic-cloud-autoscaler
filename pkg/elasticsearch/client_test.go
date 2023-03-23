package elasticsearch

import (
	"bytes"
	"context"
	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/google/go-cmp/cmp"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func newESClientWithMockResponse(status int, body string) *esv8.TypedClient {
	esClient, _ := esv8.NewTypedClient(esv8.Config{Addresses: []string{"localhost:9200"}})
	esClient.Transport = &mockTransport{
		resp: &http.Response{
			Header:     elasticsearchResponseHeader(),
			StatusCode: status,
			Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		},
		err: nil,
	}
	return esClient
}

func elasticsearchResponseHeader() map[string][]string {
	return map[string][]string{"X-Elastic-Product": {"Elasticsearch"}}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	esClient := &esv8.TypedClient{}
	want := &clientImpl{esClient: esClient}
	if got := NewClient(esClient); !reflect.DeepEqual(got, want) {
		t.Errorf("NewClient() = %v, want %v", got, want)
	}
}

func TestNodeStats_DataContentNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nodeStats *NodeStats
		want      NodeStatsNodes
	}{
		{
			name:      "node stats are empty",
			nodeStats: &NodeStats{},
			want:      nil,
		},
		{
			name: "returns data content nodes",
			nodeStats: &NodeStats{
				Nodes: map[string]*NodeStatsNode{
					"node-0001": {
						ID:    "node-0001",
						Roles: []string{"data_content", "data_hot"},
					},
					"node-0002": {
						ID:    "node-0002",
						Roles: []string{"data_warm"},
					},
					"node-0003": {
						ID:    "node-0003",
						Roles: []string{"data_content", "data_hot"},
					},
				},
			},
			want: NodeStatsNodes{
				{ID: "node-0001", Roles: []string{"data_content", "data_hot"}},
				{ID: "node-0003", Roles: []string{"data_content", "data_hot"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nodeStats.DataContentNodes(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataContentNodes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeStatsNodes_IDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    NodeStatsNodes
		want []string
	}{
		{
			name: "returns ids",
			n:    []*NodeStatsNode{{ID: "node-0001"}, {ID: "node-0002"}, {ID: "node-0003"}},
			want: []string{"node-0001", "node-0002", "node-0003"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.IDs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeStatsNodes_TotalCPUUtil(t *testing.T) {
	tests := []struct {
		name string
		n    NodeStatsNodes
		want float64
	}{
		{
			name: "returns 0 when empty",
			n:    []*NodeStatsNode{},
			want: 0,
		},
		{
			name: "150 percent in total",
			n: []*NodeStatsNode{
				{OS: &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 60}}},
				{OS: &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 50}}},
				{OS: &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 40}}},
			},
			want: 150,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.TotalCPUUtil(); got != tt.want {
				t.Errorf("TotalCPUUtil() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeStatsNodes_AvgCPUUtil(t *testing.T) {
	tests := []struct {
		name string
		n    NodeStatsNodes
		want float64
	}{
		{
			name: "50 percent on average",
			n: []*NodeStatsNode{
				{ID: "node-0001", OS: &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 60}}},
				{ID: "node-0001", OS: &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 50}}},
				{ID: "node-0001", OS: &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 40}}},
			},
			want: 50,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.AvgCPUUtil(); got != tt.want {
				t.Errorf("AvgCPUUtil() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_clientImpl_GetNodeStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		esClient *esv8.TypedClient
		want     *NodeStats
		wantErr  bool
	}{
		{
			name: "successful response",
			esClient: newESClientWithMockResponse(200, `
{
  "_nodes": {
    "total": 1,
    "successful": 1,
    "failed": 0
  },
  "cluster_name": "dummy_cluster_name",
  "nodes": {
    "dummy_node_id": {
      "timestamp": 1672190976369,
      "name": "instance-0000000001",
      "transport_address": "0.0.0.0:19481",
      "host": "0.0.0.0",
      "ip": "0.0.0.0:19481",
      "roles": [
        "data_content",
        "data_hot",
        "ingest",
        "master",
        "remote_cluster_client",
        "transform"
      ],
      "attributes": {
        "availability_zone": "us-west1-a",
        "logical_availability_zone": "zone-0",
        "xpack.installed": "true",
        "data": "hot",
        "server_name": "instance-0000000001.dummy_cluster_name",
        "instance_configuration": "gcp.es.datahot.n2.68x32x45",
        "region": "unknown-region"
      },
      "os": {
        "timestamp": 1672190976370,
        "cpu": {
          "percent": 1,
          "load_average": {
            "1m": 0.97,
            "5m": 0.53,
            "15m": 0.47
          }
        },
        "mem": {
          "total_in_bytes": 1073741824,
          "adjusted_total_in_bytes": 696254464,
          "free_in_bytes": 30466048,
          "used_in_bytes": 1043275776,
          "free_percent": 3,
          "used_percent": 97
        },
        "swap": {
          "total_in_bytes": 536870912,
          "free_in_bytes": 536809472,
          "used_in_bytes": 61440
        },
        "cgroup": {
          "cpuacct": {
            "control_group": "/",
            "usage_nanos": 14805637489730
          },
          "cpu": {
            "control_group": "/",
            "cfs_period_micros": 100000,
            "cfs_quota_micros": 800000,
            "stat": {
              "number_of_elapsed_periods": 2106495,
              "number_of_times_throttled": 240,
              "time_throttled_nanos": 35381961711
            }
          },
          "memory": {
            "control_group": "/",
            "limit_in_bytes": "1073741824",
            "usage_in_bytes": "1043275776"
          }
        }
      }
    }
  }
}`),
			want: &NodeStats{map[string]*NodeStatsNode{
				"dummy_node_id": {
					ID:    "dummy_node_id",
					Roles: []string{"data_content", "data_hot", "ingest", "master", "remote_cluster_client", "transform"},
					OS:    &NodeStatsNodeOS{CPU: &NodeStatsNodeOSCPU{Percent: 1}},
				},
			}},
		},
		{
			name:     "error response",
			esClient: newESClientWithMockResponse(500, "error"),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &clientImpl{
				esClient: tt.esClient,
			}
			got, err := c.GetNodeStats(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodeStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetNodeStats() diff(-want +got):\n%s", diff)
			}
		})
	}
}

func TestIndexSettings_TotalShardNum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		indexSettings *IndexSettings
		want          int
	}{
		{
			name: "only primary shard",
			indexSettings: &IndexSettings{
				ShardNum:   1,
				ReplicaNum: 0,
			},
			want: 1,
		},
		{
			name: "primary + replica shards",
			indexSettings: &IndexSettings{
				ShardNum:   2,
				ReplicaNum: 3,
			},
			want: 8,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.indexSettings.TotalShardNum(); got != tt.want {
				t.Errorf("TotalShardNum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_clientImpl_GetIndexSettings(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx       context.Context
		indexName string
	}
	tests := []struct {
		name     string
		esClient *esv8.TypedClient
		args     args
		want     *IndexSettings
		wantErr  bool
	}{
		{
			name: "successful response",
			args: args{
				ctx:       context.Background(),
				indexName: "test",
			},
			esClient: newESClientWithMockResponse(200, `
{
  "test": {
    "settings": {
      "index": {
        "routing": {
          "allocation": {
            "include": {
              "_tier_preference": "data_content"
            }
          }
        },
        "number_of_shards": "2",
        "auto_expand_replicas": "0-all",
        "blocks": {
          "read_only_allow_delete": "false"
        },
        "provided_name": "test",
        "creation_date": "1669733485178",
        "number_of_replicas": "3",
        "uuid": "TKzVs_BDSfe4O_pbgC1cHw",
        "version": {
          "created": "8030099"
        }
      }
    }
  }
}
`),
			want: &IndexSettings{
				ShardNum:   2,
				ReplicaNum: 3,
			},
		},
		{
			name: "error when alias index",
			args: args{
				ctx:       context.Background(),
				indexName: "test",
			},
			esClient: newESClientWithMockResponse(200, `
{
  "test-v1": {
    "settings": {
      "index": {
        "number_of_shards": "2",
        "number_of_replicas": "3"
      }
    }
  }
}
`),
			wantErr: true,
		},
		{
			name: "error response",
			args: args{
				ctx:       context.Background(),
				indexName: "test",
			},
			esClient: newESClientWithMockResponse(500, "error"),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &clientImpl{
				esClient: tt.esClient,
			}
			got, err := c.GetIndexSettings(tt.args.ctx, tt.args.indexName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIndexSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIndexSettings() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndexHealth_IsHealthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		indexHealth *IndexHealth
		want        bool
	}{
		{
			name: "healthy index",
			indexHealth: &IndexHealth{
				Status: "green",
			},
			want: true,
		},
		{
			name: "unhealthy index",
			indexHealth: &IndexHealth{
				Status:             "yellow",
				InitializingShards: 2,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.indexHealth.IsHealthy(); got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_clientImpl_GetIndexHealth(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx       context.Context
		indexName string
	}
	tests := []struct {
		name     string
		esClient *esv8.TypedClient
		args     args
		want     *IndexHealth
		wantErr  bool
	}{
		{
			name: "successful response",
			args: args{
				ctx:       context.Background(),
				indexName: "test",
			},
			esClient: newESClientWithMockResponse(200, `
{
  "cluster_name": "dummy_cluster_name",
  "status": "green",
  "timed_out": false,
  "number_of_nodes": 1,
  "number_of_data_nodes": 1,
  "active_primary_shards": 1,
  "active_shards": 1,
  "relocating_shards": 0,
  "initializing_shards": 0,
  "unassigned_shards": 0,
  "delayed_unassigned_shards": 0,
  "number_of_pending_tasks": 0,
  "number_of_in_flight_fetch": 0,
  "task_max_waiting_in_queue_millis": 0,
  "active_shards_percent_as_number": 100
}
`),
			want: &IndexHealth{
				ClusterName:                 "dummy_cluster_name",
				Status:                      "green",
				NumberOfNodes:               1,
				NumberOfDataNodes:           1,
				ActivePrimaryShards:         1,
				ActiveShards:                1,
				ActiveShardsPercentAsNumber: 100,
			},
		},
		{
			name: "error response",
			args: args{
				ctx:       context.Background(),
				indexName: "test",
			},
			esClient: newESClientWithMockResponse(500, "error"),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &clientImpl{
				esClient: tt.esClient,
			}
			got, err := c.GetIndexHealth(tt.args.ctx, tt.args.indexName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIndexHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetIndexHealth() diff(-want +got):\n%s", diff)
			}
		})
	}
}

func Test_clientImpl_UpdateIndexReplicaNum(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx        context.Context
		indexName  string
		replicaNum int
	}
	tests := []struct {
		name     string
		esClient *esv8.TypedClient
		args     args
		wantErr  bool
	}{
		{
			name: "successful response",
			args: args{
				ctx:        context.Background(),
				indexName:  "test",
				replicaNum: 2,
			},
			esClient: newESClientWithMockResponse(200, `
{
  "cluster_name": "dummy_cluster_name",
  "status": "green",
  "timed_out": false,
  "number_of_nodes": 1,
  "number_of_data_nodes": 1,
  "active_primary_shards": 1,
  "active_shards": 1,
  "relocating_shards": 0,
  "initializing_shards": 0,
  "unassigned_shards": 0,
  "delayed_unassigned_shards": 0,
  "number_of_pending_tasks": 0,
  "number_of_in_flight_fetch": 0,
  "task_max_waiting_in_queue_millis": 0,
  "active_shards_percent_as_number": 100
}
`),
		},
		{
			name: "error response",
			args: args{
				ctx:        context.Background(),
				indexName:  "test",
				replicaNum: 2,
			},
			esClient: newESClientWithMockResponse(500, "error"),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &clientImpl{
				esClient: tt.esClient,
			}
			if err := c.UpdateIndexReplicaNum(tt.args.ctx, tt.args.indexName, tt.args.replicaNum); (err != nil) != tt.wantErr {
				t.Errorf("UpdateIndexReplicaNum() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
