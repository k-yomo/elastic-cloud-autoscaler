package autoscaler

import "github.com/elastic/cloud-sdk-go/pkg/models"

type ScalingDirection string

const (
	ScalingDirectionOut  ScalingDirection = "SCALING_OUT"
	ScalingDirectionIn   ScalingDirection = "SCALING_IN"
	ScalingDirectionNone ScalingDirection = "NOT_SCALING"
)

type ScalingOperation struct {
	FromTopologySize *models.TopologySize
	ToTopologySize   *models.TopologySize
	FromReplicaNum   int
	ToReplicaNum     int
	Reason           string
}

func (s *ScalingOperation) Direction() ScalingDirection {
	switch {
	case *s.ToTopologySize.Value > *s.FromTopologySize.Value:
		return ScalingDirectionOut
	case *s.ToTopologySize.Value < *s.FromTopologySize.Value:
		return ScalingDirectionIn
	default:
		return ScalingDirectionNone
	}
}

func (s *ScalingOperation) NeedTopologySizeUpdate() bool {
	return *s.FromTopologySize.Value != *s.ToTopologySize.Value
}
