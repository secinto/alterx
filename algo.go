package alterx

// ClusterBomb generates all combinations of payloads using an Nth-order ClusterBomb algorithm.
// It uses recursion to construct permutations efficiently while avoiding stack overflows.
//
// The callback function receives each generated permutation as a map and should return true
// to continue processing or false to stop early (useful for cancellation).
//
// Algorithm Overview:
//  1. Initialize an IndexMap containing all payloads with indexed keys
//  2. Build a Vector of length n where n = len(payloads)
//  3. Use recursion to construct all possible combinations
//  4. At the final recursion level, iterate through remaining values and invoke callback
//
// Example:
//
//	Given payloads["word"] = []string{"api", "dev", "cloud"}
//	and payloads["env"] = []string{"prod", "staging"}
//	This generates: api-prod, api-staging, dev-prod, dev-staging, cloud-prod, cloud-staging
func ClusterBomb(payloads *IndexMap, callback func(varMap map[string]interface{}) bool, Vector []string) bool {
	// Base case: Vector is complete except for the last element
	if len(Vector) == payloads.Cap()-1 {
		// Construct a map with all vector elements assigned to their keys
		vectorMap := make(map[string]interface{}, payloads.Cap())
		for k, v := range Vector {
			vectorMap[payloads.KeyAtNth(k)] = v
		}

		// Fill in the final missing element and invoke callback
		index := len(Vector)
		for _, elem := range payloads.GetNth(index) {
			vectorMap[payloads.KeyAtNth(index)] = elem
			if !callback(vectorMap) {
				return false // Early termination requested
			}
		}
		return true
	}

	// Recursive case: Build up the vector by iterating through payloads at current index
	index := len(Vector)
	for _, v := range payloads.GetNth(index) {
		// Pre-allocate capacity to reduce allocations
		tmp := make([]string, len(Vector), len(Vector)+1)
		copy(tmp, Vector)
		tmp = append(tmp, v)

		if !ClusterBomb(payloads, callback, tmp) {
			return false // Propagate early termination
		}
	}
	return true
}

// IndexMap provides indexed access to a map, allowing retrieval by numeric position.
// This is useful when you need deterministic iteration order over map keys.
type IndexMap struct {
	values  map[string][]string
	indexes map[int]string
}

// GetNth returns the slice of values at the nth position in the map
func (o *IndexMap) GetNth(n int) []string {
	return o.values[o.indexes[n]]
}

// Cap returns the number of keys in the IndexMap
func (o *IndexMap) Cap() int {
	return len(o.values)
}

// KeyAtNth returns the key present at the nth position
func (o *IndexMap) KeyAtNth(n int) string {
	return o.indexes[n]
}

// NewIndexMap creates an IndexMap that allows elements to be retrieved by a fixed numeric index.
// This provides deterministic ordering for map iteration, which is useful for reproducible results.
func NewIndexMap(values map[string][]string) *IndexMap {
	i := &IndexMap{
		values:  values,
		indexes: make(map[int]string, len(values)),
	}
	counter := 0
	for k := range values {
		i.indexes[counter] = k
		counter++
	}
	return i
}
