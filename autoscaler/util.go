package autoscaler

func SixtyFourGiBNodeNumToTopologySize(nodeNum int) int32 {
	return int32(nodeNum * 64)
}
