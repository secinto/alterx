package alterx

import (
	"bytes"
	"context"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testConfig = Config{
	Patterns: []string{
		"{{sub}}-{{word}}.{{root}}", // ex: api-prod.scanme.sh
		"{{word}}-{{sub}}.{{root}}", // ex: prod-api.scanme.sh
		"{{word}}.{{sub}}.{{root}}", // ex: prod.api.scanme.sh
		"{{sub}}.{{word}}.{{root}}", // ex: api.prod.scanme.sh
	},
	Payloads: map[string][]string{
		"word": {"dev", "lib", "prod", "stage", "wp"},
	},
}

func TestMutatorCount(t *testing.T) {
	opts := &Options{
		Domains:       []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
		DedupeResults: true,
	}
	opts.Patterns = testConfig.Patterns
	opts.Payloads = testConfig.Payloads

	expectedCount := len(opts.Patterns) * len(opts.Payloads["word"]) * len(opts.Domains)
	m, err := New(opts)
	require.Nil(t, err)
	require.EqualValues(t, expectedCount, m.EstimateCount())
}

func TestMutatorResults(t *testing.T) {
	opts := &Options{
		Domains:       []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
		DedupeResults: true,
		MaxSize:       math.MaxInt,
	}
	opts.Patterns = testConfig.Patterns
	opts.Payloads = testConfig.Payloads

	m, err := New(opts)
	require.Nil(t, err)
	var buff bytes.Buffer
	err = m.ExecuteWithWriter(context.Background(), &buff)
	require.Nil(t, err)
	count := strings.Split(strings.TrimSpace(buff.String()), "\n")
	require.EqualValues(t, 80, len(count), buff.String())
}

// Comprehensive new tests below

func TestNewMutatorErrors(t *testing.T) {
	t.Run("no domains", func(t *testing.T) {
		opts := &Options{
			Domains: []string{},
		}
		_, err := New(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no domains provided")
	})

	t.Run("all invalid domains", func(t *testing.T) {
		opts := &Options{
			Domains:  []string{".com", "co.uk", "*.*.example.com"},
			Patterns: []string{"{{word}}.{{root}}"},
			Payloads: map[string][]string{"word": {"test"}},
		}
		_, err := New(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "all")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		opts := &Options{
			Domains:  []string{"example.com"},
			Patterns: []string{"{{invalid"},
			Payloads: map[string][]string{"word": {"test"}},
		}
		_, err := New(opts)
		require.Error(t, err)
	})
}

func TestMutatorWithDefaultConfig(t *testing.T) {
	t.Run("use default patterns", func(t *testing.T) {
		opts := &Options{
			Domains: []string{"example.com"},
			// Patterns empty - should use defaults
			Payloads: map[string][]string{"word": {"test"}},
		}
		m, err := New(opts)
		require.NoError(t, err)
		require.NotEmpty(t, m.Options.Patterns)
	})

	t.Run("use default payloads", func(t *testing.T) {
		opts := &Options{
			Domains:  []string{"example.com"},
			Patterns: []string{"{{word}}.{{root}}"},
			// Payloads empty - should use defaults
		}
		m, err := New(opts)
		require.NoError(t, err)
		require.NotEmpty(t, m.Options.Payloads)
	})
}

func TestMutatorLimit(t *testing.T) {
	t.Run("respect limit", func(t *testing.T) {
		opts := &Options{
			Domains:       []string{"example.com"},
			Patterns:      []string{"{{word}}.{{root}}"},
			Payloads:      map[string][]string{"word": {"a", "b", "c", "d", "e"}},
			Limit:         3,
			DedupeResults: true,
			MaxSize:       math.MaxInt,
		}
		m, err := New(opts)
		require.NoError(t, err)

		var buff bytes.Buffer
		err = m.ExecuteWithWriter(context.Background(), &buff)
		require.NoError(t, err)

		results := strings.Split(strings.TrimSpace(buff.String()), "\n")
		require.LessOrEqual(t, len(results), 3)
	})
}

func TestMutatorMaxSize(t *testing.T) {
	t.Run("respect max size", func(t *testing.T) {
		opts := &Options{
			Domains:       []string{"example.com"},
			Patterns:      []string{"{{word}}.{{root}}"},
			Payloads:      map[string][]string{"word": {"a", "b", "c", "d", "e"}},
			MaxSize:       50, // Small size to trigger limit
			DedupeResults: true,
		}
		m, err := New(opts)
		require.NoError(t, err)

		var buff bytes.Buffer
		err = m.ExecuteWithWriter(context.Background(), &buff)
		require.NoError(t, err)

		require.LessOrEqual(t, buff.Len(), 50)
	})
}

func TestMutatorEnrich(t *testing.T) {
	t.Run("enrich extracts words from domains", func(t *testing.T) {
		opts := &Options{
			Domains:       []string{"api123.example.com", "dev456.example.com"},
			Patterns:      []string{"{{word}}.{{root}}"},
			Payloads:      map[string][]string{"word": {"base"}},
			Enrich:        true,
			DedupeResults: true,
			MaxSize:       math.MaxInt,
		}
		m, err := New(opts)
		require.NoError(t, err)

		// Check that enriched words are added
		require.Contains(t, m.Options.Payloads["word"], "base")
		// Should contain extracted words
		require.Greater(t, len(m.Options.Payloads["word"]), 1)
	})
}

func TestMutatorContext(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		opts := &Options{
			Domains:       []string{"example.com"},
			Patterns:      []string{"{{word}}.{{root}}"},
			Payloads:      map[string][]string{"word": generateLargePayload(100)},
			DedupeResults: true,
			MaxSize:       math.MaxInt,
		}
		m, err := New(opts)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var buff bytes.Buffer
		err = m.ExecuteWithWriter(ctx, &buff)
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("context timeout", func(t *testing.T) {
		opts := &Options{
			Domains:       []string{"example.com"},
			Patterns:      []string{"{{word}}.{{root}}"},
			Payloads:      map[string][]string{"word": generateLargePayload(1000)},
			DedupeResults: true,
			MaxSize:       math.MaxInt,
		}
		m, err := New(opts)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout

		var buff bytes.Buffer
		err = m.ExecuteWithWriter(ctx, &buff)
		require.Error(t, err)
	})
}

func TestMutatorDeduplication(t *testing.T) {
	t.Run("with deduplication", func(t *testing.T) {
		opts := &Options{
			Domains:  []string{"example.com"},
			Patterns: []string{"{{word}}.{{root}}", "{{word}}.{{root}}"}, // Duplicate pattern
			Payloads: map[string][]string{"word": {"api"}},
			DedupeResults: true,
			MaxSize:       math.MaxInt,
		}
		m, err := New(opts)
		require.NoError(t, err)

		var buff bytes.Buffer
		err = m.ExecuteWithWriter(context.Background(), &buff)
		require.NoError(t, err)

		results := strings.Split(strings.TrimSpace(buff.String()), "\n")
		// Should deduplicate
		require.Equal(t, 1, len(results))
	})

	t.Run("without deduplication", func(t *testing.T) {
		opts := &Options{
			Domains:  []string{"example.com"},
			Patterns: []string{"{{word}}.{{root}}", "{{word}}.{{root}}"}, // Duplicate pattern
			Payloads: map[string][]string{"word": {"api"}},
			DedupeResults: false,
			MaxSize:       math.MaxInt,
		}
		m, err := New(opts)
		require.NoError(t, err)

		var buff bytes.Buffer
		err = m.ExecuteWithWriter(context.Background(), &buff)
		require.NoError(t, err)

		results := strings.Split(strings.TrimSpace(buff.String()), "\n")
		// Should NOT deduplicate
		require.Equal(t, 2, len(results))
	})
}

func TestMutatorDryRun(t *testing.T) {
	opts := &Options{
		Domains:       []string{"example.com"},
		Patterns:      []string{"{{word}}.{{root}}"},
		Payloads:      map[string][]string{"word": {"a", "b", "c"}},
		DedupeResults: true,
		MaxSize:       math.MaxInt,
	}
	m, err := New(opts)
	require.NoError(t, err)

	count := m.DryRun()
	require.Equal(t, 3, count)
}

func TestMutatorPayloadCount(t *testing.T) {
	opts := &Options{
		Domains:       []string{"example.com"},
		Patterns:      []string{"{{word}}.{{root}}"},
		Payloads:      map[string][]string{"word": {"a", "b"}},
		DedupeResults: true,
		MaxSize:       math.MaxInt,
	}
	m, err := New(opts)
	require.NoError(t, err)

	// Before execution
	require.Equal(t, 2, m.PayloadCount())

	// After execution
	var buff bytes.Buffer
	err = m.ExecuteWithWriter(context.Background(), &buff)
	require.NoError(t, err)
	require.Equal(t, 2, m.PayloadCount())
}

func TestMutatorSkipsInvalidDomains(t *testing.T) {
	opts := &Options{
		Domains:       []string{"valid.example.com", ".invalid", "another.example.com"},
		Patterns:      []string{"{{word}}.{{root}}"},
		Payloads:      map[string][]string{"word": {"api"}},
		DedupeResults: true,
		MaxSize:       math.MaxInt,
	}
	m, err := New(opts)
	require.NoError(t, err)

	// Should skip .invalid but process the others
	require.Equal(t, 2, len(m.Inputs))

	var buff bytes.Buffer
	err = m.ExecuteWithWriter(context.Background(), &buff)
	require.NoError(t, err)

	results := strings.Split(strings.TrimSpace(buff.String()), "\n")
	require.Equal(t, 2, len(results))
}

func TestMutatorNilWriter(t *testing.T) {
	opts := &Options{
		Domains:       []string{"example.com"},
		Patterns:      []string{"{{word}}.{{root}}"},
		Payloads:      map[string][]string{"word": {"api"}},
		DedupeResults: true,
	}
	m, err := New(opts)
	require.NoError(t, err)

	err = m.ExecuteWithWriter(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "writer destination cannot be nil")
}

func TestMutatorSkipsHyphenPrefixedResults(t *testing.T) {
	// This tests that results starting with "-" are skipped (invalid domains)
	opts := &Options{
		Domains:       []string{"example.com"},
		Patterns:      []string{"{{word}}.{{root}}"},
		Payloads:      map[string][]string{"word": {"-invalid", "valid"}},
		DedupeResults: true,
		MaxSize:       math.MaxInt,
	}
	m, err := New(opts)
	require.NoError(t, err)

	var buff bytes.Buffer
	err = m.ExecuteWithWriter(context.Background(), &buff)
	require.NoError(t, err)

	results := buff.String()
	require.NotContains(t, results, "-invalid.example.com")
	require.Contains(t, results, "valid.example.com")
}

// Helper functions

func generateLargePayload(size int) []string {
	payload := make([]string, size)
	for i := 0; i < size; i++ {
		payload[i] = string(rune('a' + (i % 26)))
	}
	return payload
}

// Benchmarks

func BenchmarkMutatorNew(b *testing.B) {
	opts := &Options{
		Domains:       []string{"api.example.com", "dev.example.com", "prod.example.com"},
		Patterns:      testConfig.Patterns,
		Payloads:      testConfig.Payloads,
		DedupeResults: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = New(opts)
	}
}

func BenchmarkMutatorExecute(b *testing.B) {
	opts := &Options{
		Domains:       []string{"example.com"},
		Patterns:      testConfig.Patterns,
		Payloads:      testConfig.Payloads,
		DedupeResults: true,
		MaxSize:       math.MaxInt,
	}
	m, _ := New(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := m.ExecuteWithWriter(context.Background(), io.Discard)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMutatorEstimateCount(b *testing.B) {
	opts := &Options{
		Domains:       []string{"example.com"},
		Patterns:      testConfig.Patterns,
		Payloads:      testConfig.Payloads,
		DedupeResults: true,
	}
	m, _ := New(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.EstimateCount()
	}
}
