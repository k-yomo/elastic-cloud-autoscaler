package autoscaler

func SixtyFourGiBNodeNumToTopologySize(nodeNum int) int {
	return nodeNum * 64
}
