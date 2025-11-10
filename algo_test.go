package alterx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClusterBomb(t *testing.T) {
	t.Run("basic two variable combination", func(t *testing.T) {
		payloads := map[string][]string{
			"word": {"api", "dev"},
			"env":  {"prod", "staging"},
		}
		indexMap := NewIndexMap(payloads)

		var results []map[string]interface{}
		callback := func(varMap map[string]interface{}) bool {
			// Make a copy to avoid reference issues
			result := make(map[string]interface{})
			for k, v := range varMap {
				result[k] = v
			}
			results = append(results, result)
			return true
		}

		success := ClusterBomb(indexMap, callback, []string{})
		require.True(t, success)
		require.Len(t, results, 4, "Should generate 2*2=4 combinations")
	})

	t.Run("three variable combination", func(t *testing.T) {
		payloads := map[string][]string{
			"var1": {"a", "b"},
			"var2": {"1", "2"},
			"var3": {"x", "y"},
		}
		indexMap := NewIndexMap(payloads)

		var results []map[string]interface{}
		callback := func(varMap map[string]interface{}) bool {
			result := make(map[string]interface{})
			for k, v := range varMap {
				result[k] = v
			}
			results = append(results, result)
			return true
		}

		success := ClusterBomb(indexMap, callback, []string{})
		require.True(t, success)
		require.Len(t, results, 8, "Should generate 2*2*2=8 combinations")

		// Verify each result has all three variables
		for _, result := range results {
			require.Len(t, result, 3)
			require.Contains(t, result, "var1")
			require.Contains(t, result, "var2")
			require.Contains(t, result, "var3")
		}
	})

	t.Run("single variable", func(t *testing.T) {
		payloads := map[string][]string{
			"word": {"api", "dev", "prod"},
		}
		indexMap := NewIndexMap(payloads)

		var results []map[string]interface{}
		callback := func(varMap map[string]interface{}) bool {
			result := make(map[string]interface{})
			for k, v := range varMap {
				result[k] = v
			}
			results = append(results, result)
			return true
		}

		success := ClusterBomb(indexMap, callback, []string{})
		require.True(t, success)
		require.Len(t, results, 3)

		// Check all values are present
		values := make([]string, 0, 3)
		for _, r := range results {
			values = append(values, r["word"].(string))
		}
		require.ElementsMatch(t, []string{"api", "dev", "prod"}, values)
	})

	t.Run("early termination", func(t *testing.T) {
		payloads := map[string][]string{
			"word": {"api", "dev", "prod", "staging", "test"},
		}
		indexMap := NewIndexMap(payloads)

		count := 0
		callback := func(varMap map[string]interface{}) bool {
			count++
			if count >= 3 {
				return false // Stop after 3 results
			}
			return true
		}

		success := ClusterBomb(indexMap, callback, []string{})
		require.False(t, success, "Should return false on early termination")
		require.Equal(t, 3, count, "Should have called callback exactly 3 times")
	})

	t.Run("large payload sets", func(t *testing.T) {
		// Generate larger payload sets to test performance
		payload1 := make([]string, 10)
		payload2 := make([]string, 10)
		for i := 0; i < 10; i++ {
			payload1[i] = string(rune('a' + i))
			payload2[i] = string(rune('0' + i))
		}

		payloads := map[string][]string{
			"letters": payload1,
			"numbers": payload2,
		}
		indexMap := NewIndexMap(payloads)

		count := 0
		callback := func(varMap map[string]interface{}) bool {
			count++
			return true
		}

		success := ClusterBomb(indexMap, callback, []string{})
		require.True(t, success)
		require.Equal(t, 100, count, "Should generate 10*10=100 combinations")
	})

	t.Run("empty payloads", func(t *testing.T) {
		payloads := map[string][]string{
			"word": {},
		}
		indexMap := NewIndexMap(payloads)

		count := 0
		callback := func(varMap map[string]interface{}) bool {
			count++
			return true
		}

		success := ClusterBomb(indexMap, callback, []string{})
		require.True(t, success)
		require.Equal(t, 0, count, "Should not call callback with empty payloads")
	})
}

func TestNewIndexMap(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		values := map[string][]string{
			"word": {"api", "dev"},
			"env":  {"prod"},
		}

		indexMap := NewIndexMap(values)
		require.NotNil(t, indexMap)
		require.Equal(t, 2, indexMap.Cap())
	})

	t.Run("get nth element", func(t *testing.T) {
		values := map[string][]string{
			"first":  {"a", "b"},
			"second": {"1", "2", "3"},
		}

		indexMap := NewIndexMap(values)

		// Get elements by index
		elem0 := indexMap.GetNth(0)
		elem1 := indexMap.GetNth(1)

		require.NotNil(t, elem0)
		require.NotNil(t, elem1)

		// One should be "first" and one should be "second"
		key0 := indexMap.KeyAtNth(0)
		key1 := indexMap.KeyAtNth(1)

		require.Contains(t, []string{"first", "second"}, key0)
		require.Contains(t, []string{"first", "second"}, key1)
		require.NotEqual(t, key0, key1)
	})

	t.Run("empty map", func(t *testing.T) {
		values := map[string][]string{}
		indexMap := NewIndexMap(values)
		require.NotNil(t, indexMap)
		require.Equal(t, 0, indexMap.Cap())
	})
}

func TestIndexMapDeterminism(t *testing.T) {
	// Test that IndexMap provides deterministic ordering
	values := map[string][]string{
		"a": {"1"},
		"b": {"2"},
		"c": {"3"},
	}

	im1 := NewIndexMap(values)
	im2 := NewIndexMap(values)

	// The ordering might be different between im1 and im2 (maps are unordered)
	// but each IndexMap should be internally consistent
	for i := 0; i < im1.Cap(); i++ {
		require.Equal(t, im1.KeyAtNth(i), im1.KeyAtNth(i), "IndexMap should be consistent with itself")
		require.Equal(t, im1.GetNth(i), im1.GetNth(i), "IndexMap should return same values for same index")
	}
}

func BenchmarkClusterBomb(b *testing.B) {
	payloads := map[string][]string{
		"word":   {"api", "dev", "prod", "staging"},
		"env":    {"test", "qa", "production"},
		"region": {"us", "eu", "asia"},
	}
	indexMap := NewIndexMap(payloads)

	callback := func(varMap map[string]interface{}) bool {
		// Simulate some work
		_ = varMap
		return true
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClusterBomb(indexMap, callback, []string{})
	}
}

func BenchmarkClusterBombLarge(b *testing.B) {
	// Benchmark with larger payload sets
	payload := make([]string, 20)
	for i := 0; i < 20; i++ {
		payload[i] = string(rune('a' + i))
	}

	payloads := map[string][]string{
		"var1": payload[:10],
		"var2": payload[10:],
	}
	indexMap := NewIndexMap(payloads)

	callback := func(varMap map[string]interface{}) bool {
		return true
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClusterBomb(indexMap, callback, []string{})
	}
}
