package alterx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVarCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"no variables", "static.example.com", 0},
		{"single variable", "{{word}}.example.com", 1},
		{"two variables", "{{sub}}.{{word}}.com", 2},
		{"three variables", "{{sub}}.{{word}}.{{root}}", 3},
		{"repeated variable", "{{word}}.{{word}}.com", 2},
		{"complex pattern", "{{sub}}-{{word}}-{{number}}.{{root}}", 4},
		{"empty string", "", 0},
		{"malformed brackets", "{{word}.example.com", 0},
		{"single brackets", "{word}.example.com", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVarCount(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAllVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "no variables",
			input:    "static.example.com",
			expected: nil,
		},
		{
			name:     "single variable",
			input:    "{{word}}.example.com",
			expected: []string{"word"},
		},
		{
			name:     "multiple variables",
			input:    "{{sub}}.{{word}}.{{root}}",
			expected: []string{"sub", "word", "root"},
		},
		{
			name:     "repeated variable",
			input:    "{{word}}.{{word}}.com",
			expected: []string{"word", "word"},
		},
		{
			name:     "alphanumeric variables",
			input:    "{{sub1}}.{{sub2}}.{{word3}}",
			expected: []string{"sub1", "sub2", "word3"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "malformed pattern",
			input:    "{{word.example.com",
			expected: nil,
		},
		{
			name:     "mixed valid and invalid",
			input:    "{{valid}}.{invalid}.{{another}}",
			expected: []string{"valid", "another"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAllVars(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSampleMap(t *testing.T) {
	t.Run("combine input and payload vars", func(t *testing.T) {
		inputVars := map[string]interface{}{
			"sub":  "api",
			"root": "example.com",
		}
		payloadVars := map[string][]string{
			"word":   {"dev", "prod"},
			"number": {"1", "2"},
		}

		result := getSampleMap(inputVars, payloadVars)

		require.Contains(t, result, "sub")
		require.Contains(t, result, "root")
		require.Contains(t, result, "word")
		require.Contains(t, result, "number")

		require.Equal(t, "api", result["sub"])
		require.Equal(t, "example.com", result["root"])
		require.Equal(t, "temp", result["word"])
		require.Equal(t, "temp", result["number"])
	})

	t.Run("empty payload vars", func(t *testing.T) {
		inputVars := map[string]interface{}{
			"sub": "api",
		}
		payloadVars := map[string][]string{}

		result := getSampleMap(inputVars, payloadVars)

		require.Len(t, result, 1)
		require.Contains(t, result, "sub")
	})

	t.Run("empty input vars", func(t *testing.T) {
		inputVars := map[string]interface{}{}
		payloadVars := map[string][]string{
			"word": {"dev"},
		}

		result := getSampleMap(inputVars, payloadVars)

		require.Contains(t, result, "word")
		require.Equal(t, "temp", result["word"])
	})

	t.Run("empty payload value should be skipped", func(t *testing.T) {
		inputVars := map[string]interface{}{}
		payloadVars := map[string][]string{
			"word":  {"dev"},
			"empty": {},
		}

		result := getSampleMap(inputVars, payloadVars)

		require.Contains(t, result, "word")
		require.NotContains(t, result, "empty", "Empty payload should not be included")
	})

	t.Run("empty key should be skipped", func(t *testing.T) {
		inputVars := map[string]interface{}{}
		payloadVars := map[string][]string{
			"":     {"value"},
			"word": {"dev"},
		}

		result := getSampleMap(inputVars, payloadVars)

		require.Contains(t, result, "word")
		require.NotContains(t, result, "")
	})
}

func TestCheckMissing(t *testing.T) {
	t.Run("all variables present", func(t *testing.T) {
		template := "{{word}}.{{root}}"
		data := map[string]interface{}{
			"word": "api",
			"root": "example.com",
		}

		err := checkMissing(template, data)
		require.NoError(t, err)
	})

	t.Run("missing variable", func(t *testing.T) {
		template := "{{word}}.{{missing}}.{{root}}"
		data := map[string]interface{}{
			"word": "api",
			"root": "example.com",
		}

		err := checkMissing(template, data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "{{missing}}")
	})

	t.Run("multiple missing variables", func(t *testing.T) {
		template := "{{word}}.{{missing1}}.{{missing2}}.{{root}}"
		data := map[string]interface{}{
			"word": "api",
			"root": "example.com",
		}

		err := checkMissing(template, data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "{{missing1}}")
		require.Contains(t, err.Error(), "{{missing2}}")
	})

	t.Run("no variables in template", func(t *testing.T) {
		template := "static.example.com"
		data := map[string]interface{}{
			"word": "api",
		}

		err := checkMissing(template, data)
		require.NoError(t, err)
	})

	t.Run("empty data map", func(t *testing.T) {
		template := "{{word}}.example.com"
		data := map[string]interface{}{}

		err := checkMissing(template, data)
		require.Error(t, err)
	})

	t.Run("extra variables in data", func(t *testing.T) {
		template := "{{word}}.example.com"
		data := map[string]interface{}{
			"word":  "api",
			"extra": "unused",
		}

		err := checkMissing(template, data)
		require.NoError(t, err, "Extra variables should not cause error")
	})
}

func TestUnsafeToBytes(t *testing.T) {
	t.Run("basic string conversion", func(t *testing.T) {
		str := "hello world"
		bytes := unsafeToBytes(str)

		require.Equal(t, []byte("hello world"), bytes)
		require.Equal(t, len(str), len(bytes))
	})

	t.Run("empty string", func(t *testing.T) {
		str := ""
		bytes := unsafeToBytes(str)

		require.Empty(t, bytes)
		require.Equal(t, 0, len(bytes))
	})

	t.Run("unicode characters", func(t *testing.T) {
		str := "hello 世界"
		bytes := unsafeToBytes(str)

		require.Equal(t, []byte(str), bytes)
	})

	t.Run("special characters", func(t *testing.T) {
		str := "test!@#$%^&*()_+-=[]{}|;':\",./<>?"
		bytes := unsafeToBytes(str)

		require.Equal(t, []byte(str), bytes)
	})

	t.Run("long string", func(t *testing.T) {
		str := "this is a very long string that contains many characters to test the conversion"
		bytes := unsafeToBytes(str)

		require.Equal(t, []byte(str), bytes)
		require.Equal(t, len(str), len(bytes))
	})
}

func BenchmarkGetAllVars(b *testing.B) {
	template := "{{sub}}.{{sub1}}.{{sub2}}-{{word}}-{{number}}.{{root}}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getAllVars(template)
	}
}

func BenchmarkCheckMissing(b *testing.B) {
	template := "{{sub}}-{{word}}.{{root}}"
	data := map[string]interface{}{
		"sub":  "api",
		"word": "dev",
		"root": "example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checkMissing(template, data)
	}
}

func BenchmarkUnsafeToBytes(b *testing.B) {
	str := "api-dev.example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = unsafeToBytes(str)
	}
}
