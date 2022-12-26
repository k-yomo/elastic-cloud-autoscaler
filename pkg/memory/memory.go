package memory

func ConvertGiBToMiB(gib int32) int32 {
	return gib * 1024
}

func ConvertMibToGiB(mib int32) int32 {
	return mib / 1024
}
