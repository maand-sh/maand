package utils

func Unique(vals []string) []string {
	uniqueMap := make(map[string]struct{})
	for _, val := range vals {
		uniqueMap[val] = struct{}{}
	}
	uniqueVals := make([]string, 0, len(uniqueMap))
	for key := range uniqueMap {
		uniqueVals = append(uniqueVals, key)
	}
	return uniqueVals
}

func Union(set1, set2 []string) []string {
	uniqueMap := make(map[string]struct{})
	for _, val := range set1 {
		uniqueMap[val] = struct{}{}
	}
	for _, val := range set2 {
		uniqueMap[val] = struct{}{}
	}
	unionVals := make([]string, 0, len(uniqueMap))
	for key := range uniqueMap {
		unionVals = append(unionVals, key)
	}
	return unionVals
}

func Intersection(set1, set2 []string) []string {
	intersectionVals := make([]string, 0)
	vals1Map := make(map[string]struct{})
	for _, val := range set1 {
		vals1Map[val] = struct{}{}
	}
	for _, val := range set2 {
		if _, found := vals1Map[val]; found {
			intersectionVals = append(intersectionVals, val)
		}
	}
	return intersectionVals
}

func Difference(set1, set2 []string) []string {
	vals2Map := make(map[string]struct{})
	for _, val := range set2 {
		vals2Map[val] = struct{}{}
	}
	differenceVals := make([]string, 0)
	for _, val := range set1 {
		if _, found := vals2Map[val]; !found {
			differenceVals = append(differenceVals, val)
		}
	}
	return differenceVals
}
