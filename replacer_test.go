package alterx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplace(t *testing.T) {
	t.Run("basic replacement", func(t *testing.T) {
		template := "{{word}}.example.com"
		values := map[string]interface{}{
			"word": "api",
		}

		result := Replace(template, values)
		require.Equal(t, "api.example.com", result)
	})

	t.Run("multiple variables", func(t *testing.T) {
		template := "{{sub}}-{{word}}.{{root}}"
		values := map[string]interface{}{
			"sub":  "api",
			"word": "dev",
			"root": "example.com",
		}

		result := Replace(template, values)
		require.Equal(t, "api-dev.example.com", result)
	})

	t.Run("no variables", func(t *testing.T) {
		template := "static.example.com"
		values := map[string]interface{}{}

		result := Replace(template, values)
		require.Equal(t, "static.example.com", result)
	})

	t.Run("unused variables in map", func(t *testing.T) {
		template := "{{word}}.example.com"
		values := map[string]interface{}{
			"word":   "api",
			"unused": "value",
			"extra":  "data",
		}

		result := Replace(template, values)
		require.Equal(t, "api.example.com", result)
	})

	t.Run("missing variable leaves placeholder", func(t *testing.T) {
		template := "{{word}}.{{missing}}.example.com"
		values := map[string]interface{}{
			"word": "api",
		}

		result := Replace(template, values)
		// Missing variables are not replaced
		require.Contains(t, result, "api")
		require.Contains(t, result, "{{missing}}")
	})

	t.Run("numeric values", func(t *testing.T) {
		template := "server-{{number}}.example.com"
		values := map[string]interface{}{
			"number": 123,
		}

		result := Replace(template, values)
		require.Equal(t, "server-123.example.com", result)
	})

	t.Run("boolean values", func(t *testing.T) {
		template := "{{value}}-test.com"
		values := map[string]interface{}{
			"value": true,
		}

		result := Replace(template, values)
		require.Equal(t, "true-test.com", result)
	})

	t.Run("general marker replacement", func(t *testing.T) {
		// Test the General marker ("§") functionality
		template := "§word§.example.com"
		values := map[string]interface{}{
			"word": "api",
		}

		result := Replace(template, values)
		require.Equal(t, "api.example.com", result)
	})

	t.Run("mixed markers", func(t *testing.T) {
		template := "{{word}}.§env§.example.com"
		values := map[string]interface{}{
			"word": "api",
			"env":  "prod",
		}

		result := Replace(template, values)
		require.Equal(t, "api.prod.example.com", result)
	})

	t.Run("empty template", func(t *testing.T) {
		template := ""
		values := map[string]interface{}{
			"word": "api",
		}

		result := Replace(template, values)
		require.Equal(t, "", result)
	})

	t.Run("empty values", func(t *testing.T) {
		template := "{{word}}.example.com"
		values := map[string]interface{}{}

		result := Replace(template, values)
		require.Equal(t, "{{word}}.example.com", result)
	})

	t.Run("special characters in values", func(t *testing.T) {
		template := "{{word}}.example.com"
		values := map[string]interface{}{
			"word": "api-v1.2",
		}

		result := Replace(template, values)
		require.Equal(t, "api-v1.2.example.com", result)
	})

	t.Run("repeated variables", func(t *testing.T) {
		template := "{{word}}.{{word}}.example.com"
		values := map[string]interface{}{
			"word": "api",
		}

		result := Replace(template, values)
		require.Equal(t, "api.api.example.com", result)
	})

	t.Run("nested-like syntax", func(t *testing.T) {
		template := "{{sub{{nested}}}}.example.com"
		values := map[string]interface{}{
			"sub{{nested}}": "value",
		}

		result := Replace(template, values)
		require.Equal(t, "value.example.com", result)
	})

	t.Run("whitespace in placeholders", func(t *testing.T) {
		template := "{{ word }}.example.com"
		values := map[string]interface{}{
			" word ": "api",
		}

		result := Replace(template, values)
		require.Equal(t, "api.example.com", result)
	})
}

func BenchmarkReplace(b *testing.B) {
	template := "{{sub}}-{{word}}.{{root}}"
	values := map[string]interface{}{
		"sub":  "api",
		"word": "dev",
		"root": "example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Replace(template, values)
	}
}

func BenchmarkReplaceComplex(b *testing.B) {
	template := "{{sub}}.{{sub1}}.{{sub2}}-{{word}}-{{number}}.{{sld}}.{{etld}}"
	values := map[string]interface{}{
		"sub":    "api",
		"sub1":   "v1",
		"sub2":   "internal",
		"word":   "service",
		"number": "123",
		"sld":    "example",
		"etld":   "co.uk",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Replace(template, values)
	}
}
