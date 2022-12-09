package elasticcloud

import (
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/memory"
)

func FindHotContentTopology(topologies []*models.ElasticsearchClusterTopologyElement) *models.ElasticsearchClusterTopologyElement {
	for _, topology := range topologies {
		if TopologyID(topology.ID) == TopologyIDHotContent {
			return topology
		}
	}
	panic("hot_content topology must exist")
}

func CalcTopologyNodeNum(topology *models.ElasticsearchClusterTopologyElement) int {
	sixtyFourGB := memory.ConvertGiBToMiB(64)
	if *topology.Size.Value <= sixtyFourGB {
		return int(topology.ZoneCount)
	}
	return int(*topology.Size.Value / sixtyFourGB * topology.ZoneCount)
}
