package alterx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	t.Run("valid config with patterns and payloads", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `patterns:
  - "{{sub}}-{{word}}.{{root}}"
  - "{{word}}.{{sub}}.{{root}}"
payloads:
  word:
    - dev
    - api
    - prod
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := NewConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		require.Len(t, cfg.Patterns, 2)
		require.Equal(t, "{{sub}}-{{word}}.{{root}}", cfg.Patterns[0])
		require.Equal(t, "{{word}}.{{sub}}.{{root}}", cfg.Patterns[1])

		require.Contains(t, cfg.Payloads, "word")
		require.ElementsMatch(t, []string{"dev", "api", "prod"}, cfg.Payloads["word"])
	})

	t.Run("config with wordlist file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create wordlist file
		wordlistPath := filepath.Join(tmpDir, "words.txt")
		wordlistContent := "api\ndev\nprod\nstaging"
		err := os.WriteFile(wordlistPath, []byte(wordlistContent), 0644)
		require.NoError(t, err)

		// Create config that references wordlist
		configPath := filepath.Join(tmpDir, "config.yaml")
		configContent := `patterns:
  - "{{word}}.{{root}}"
payloads:
  word:
    - ` + wordlistPath + `
`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := NewConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		require.Contains(t, cfg.Payloads, "word")
		require.ElementsMatch(t, []string{"api", "dev", "prod", "staging"}, cfg.Payloads["word"])
	})

	t.Run("config with mixed inline and file payloads", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create wordlist file
		wordlistPath := filepath.Join(tmpDir, "extra.txt")
		err := os.WriteFile(wordlistPath, []byte("extra1\nextra2"), 0644)
		require.NoError(t, err)

		// Create config with both inline and file payloads
		configPath := filepath.Join(tmpDir, "config.yaml")
		configContent := `patterns:
  - "test"
payloads:
  word:
    - inline1
    - inline2
    - ` + wordlistPath + `
`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := NewConfig(configPath)
		require.NoError(t, err)
		require.Contains(t, cfg.Payloads, "word")
		require.ElementsMatch(t, []string{"inline1", "inline2", "extra1", "extra2"}, cfg.Payloads["word"])
	})

	t.Run("nonexistent config file", func(t *testing.T) {
		cfg, err := NewConfig("/nonexistent/path/config.yaml")
		require.Error(t, err)
		require.Nil(t, cfg)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "invalid.yaml")

		invalidContent := `patterns:
  - "test"
payloads:
  this is not valid yaml: [[[
`
		err := os.WriteFile(configPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		cfg, err := NewConfig(configPath)
		require.Error(t, err)
		require.Nil(t, cfg)
	})

	t.Run("config with nonexistent wordlist file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `patterns:
  - "test"
payloads:
  word:
    - /nonexistent/wordlist.txt
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := NewConfig(configPath)
		require.NoError(t, err, "Should not fail, just skip missing wordlist file")
		// The word payload should be empty or not contain the file
		if words, ok := cfg.Payloads["word"]; ok {
			require.NotContains(t, words, "/nonexistent/wordlist.txt")
		}
	})

	t.Run("empty config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "empty.yaml")

		err := os.WriteFile(configPath, []byte(""), 0644)
		require.NoError(t, err)

		cfg, err := NewConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Empty(t, cfg.Patterns)
		require.Empty(t, cfg.Payloads)
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("default config is loaded", func(t *testing.T) {
		require.NotNil(t, DefaultConfig)
		require.NotEmpty(t, DefaultConfig.Patterns, "Default config should have patterns")
		require.NotEmpty(t, DefaultConfig.Payloads, "Default config should have payloads")
	})

	t.Run("default config has word payloads", func(t *testing.T) {
		require.Contains(t, DefaultConfig.Payloads, "word")
		require.NotEmpty(t, DefaultConfig.Payloads["word"], "Default word payloads should not be empty")
	})

	t.Run("default patterns are valid", func(t *testing.T) {
		// Check that patterns contain expected variable syntax
		for _, pattern := range DefaultConfig.Patterns {
			require.NotEmpty(t, pattern)
			// Patterns should contain variable placeholders
			require.Contains(t, pattern, "{{")
			require.Contains(t, pattern, "}}")
		}
	})
}

func TestConfigMultiplePayloads(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `patterns:
  - "{{word}}-{{number}}.{{root}}"
payloads:
  word:
    - api
    - dev
  number:
    - "1"
    - "2"
    - "3"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := NewConfig(configPath)
	require.NoError(t, err)

	require.Contains(t, cfg.Payloads, "word")
	require.Contains(t, cfg.Payloads, "number")
	require.Len(t, cfg.Payloads["word"], 2)
	require.Len(t, cfg.Payloads["number"], 3)
}
