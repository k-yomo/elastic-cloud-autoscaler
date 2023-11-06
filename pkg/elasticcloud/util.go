package elasticcloud

import (
	"fmt"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/memory"
	"strings"
)

func FindHotContentTopology(topologies []*models.ElasticsearchClusterTopologyElement) *models.ElasticsearchClusterTopologyElement {
	for _, topology := range topologies {
		if TopologyID(topology.ID) == TopologyIDHotContent {
			return topology
		}
	}
	panic("hot_content topology must exist")
}

func CalcNodeNum(topologySize *models.TopologySize, zoneCount int32) int {
	sixtyFourGB := memory.ConvertGiBToMiB(64)
	if *topologySize.Value <= sixtyFourGB {
		return int(zoneCount)
	}
	return int(*topologySize.Value / sixtyFourGB * zoneCount)
}

func CalcTopologyNodeNum(topology *models.ElasticsearchClusterTopologyElement) int {
	return CalcNodeNum(topology.Size, topology.ZoneCount)
}

func formatPlanAttemptError(planAttemptError *models.ClusterPlanAttemptError) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "failureType: %s", planAttemptError.FailureType)
	if planAttemptError.Message != nil {
		fmt.Fprintf(&sb, ", message: %s", *planAttemptError.Message)
	}
	if planAttemptError.Timestamp != nil {
		fmt.Fprintf(&sb, ", timestamp: %s", planAttemptError.Timestamp.String())
	}
	fmt.Fprintf(&sb, ", details: %v", planAttemptError.Details)
	return sb.String()
}
