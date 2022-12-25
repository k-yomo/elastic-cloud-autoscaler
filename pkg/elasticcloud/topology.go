package elasticcloud

import (
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/util/ec"
	"github.com/k-yomo/elastic-cloud-autoscaler/pkg/memory"
)

type TopologyID string

const (
	TopologyIDHotContent   TopologyID = "hot_content"
	TopologyIDWarm         TopologyID = "warm"
	TopologyIDCold         TopologyID = "cold"
	TopologyIDCoordinating TopologyID = "coordinating"
	TopologyIDFrozen       TopologyID = "frozen"
	TopologyIDML           TopologyID = "ml"
	TopologyIDMaster       TopologyID = "master"
)

func NewTopologySize(gib int32) *models.TopologySize {
	return &models.TopologySize{
		Resource: ec.String("memory"),
		Value:    ec.Int32(memory.ConvertGiBToMiB(gib)),
	}
}
