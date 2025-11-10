package alterx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInput(t *testing.T) {
	testcases := []string{"scanme.co.uk", "https://scanme.co.uk", "scanme.co.uk:443", "https://scanme.co.uk:443"}
	expected := &Input{
		TLD:    "uk",
		ETLD:   "co.uk",
		SLD:    "scanme",
		Root:   "scanme.co.uk",
		Suffix: "scanme.co.uk",
		Sub:    "",
	}
	for _, v := range testcases {
		got, err := NewInput(v)
		require.Nilf(t, err, "failed to parse url %v", v)
		require.Equal(t, expected, got)
	}
}

func TestInputSub(t *testing.T) {
	testcases := []struct {
		url      string
		expected *Input
	}{
		{url: "something.scanme.sh", expected: &Input{TLD: "sh", ETLD: "", SLD: "scanme", Root: "scanme.sh", Sub: "something", Suffix: "scanme.sh"}},
		{url: "nested.something.scanme.sh", expected: &Input{TLD: "sh", ETLD: "", SLD: "scanme", Root: "scanme.sh", Sub: "nested", Suffix: "something.scanme.sh", MultiLevel: []string{"something"}}},
		{url: "nested.multilevel.scanme.co.uk", expected: &Input{TLD: "uk", ETLD: "co.uk", SLD: "scanme", Root: "scanme.co.uk", Sub: "nested", Suffix: "multilevel.scanme.co.uk", MultiLevel: []string{"multilevel"}}},
		{url: "sub.level1.level2.scanme.sh", expected: &Input{TLD: "sh", ETLD: "", SLD: "scanme", Root: "scanme.sh", Sub: "sub", Suffix: "level1.level2.scanme.sh", MultiLevel: []string{"level1", "level2"}}},
		{url: "scanme.sh", expected: &Input{TLD: "sh", ETLD: "", Sub: "", Suffix: "scanme.sh", SLD: "scanme", Root: "scanme.sh"}},
	}
	for _, v := range testcases {
		got, err := NewInput(v.url)
		require.Nilf(t, err, "failed to parse url %v", v.url)
		require.Equal(t, v.expected, got, *v.expected)
	}
}

func TestVarCount(t *testing.T) {
	testcases := []struct {
		statement string
		count     int
	}{
		{statement: "{{sub}}.something.{{tld}}", count: 2},
		{statement: "{{sub}}.{{sub1}}.{{sub2}}.{{root}}", count: 4},
		{statement: "no variables", count: 0},
	}
	for _, v := range testcases {
		require.EqualValues(t, v.count, getVarCount(v.statement), "variable count mismatch")
	}
}

func TestExtractVar(t *testing.T) {
	// extract all variables from statement
	testcases := []struct {
		statement string
		expected  []string
	}{
		{statement: "{{sub}}.something.{{tld}}", expected: []string{"sub", "tld"}},
		{statement: "{{sub}}.{{sub1}}.{{sub2}}.{{root}}", expected: []string{"sub", "sub1", "sub2", "root"}},
		{statement: "no variables", expected: nil},
	}
	for _, v := range testcases {
		actual := getAllVars(v.statement)
		require.Equal(t, v.expected, actual)
	}
}

// New comprehensive tests below

func TestInputErrors(t *testing.T) {
	testcases := []struct {
		name  string
		input string
	}{
		{"invalid url", "ht!tp://invalid"},
		{"just tld", ".com"},
		{"just public suffix", "co.uk"},
		{"wildcard in middle", "api.*.example.com"},
		{"multiple wildcards", "*.*.example.com"},
		{"empty string", ""},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewInput(tc.input)
			require.Error(t, err, "Expected error for input: %s", tc.input)
		})
	}
}

func TestInputWildcard(t *testing.T) {
	t.Run("leading wildcard", func(t *testing.T) {
		input, err := NewInput("*.example.com")
		require.NoError(t, err)
		require.Equal(t, "example.com", input.Root)
		require.Equal(t, "example", input.SLD)
		require.Equal(t, "com", input.TLD)
	})

	t.Run("leading wildcard with subdomain", func(t *testing.T) {
		input, err := NewInput("*.api.example.com")
		require.NoError(t, err)
		require.Equal(t, "example.com", input.Root)
		require.Equal(t, "api", input.Sub)
	})
}

func TestInputEdgeCases(t *testing.T) {
	t.Run("very deep subdomain", func(t *testing.T) {
		input, err := NewInput("a.b.c.d.e.f.example.com")
		require.NoError(t, err)
		require.Equal(t, "a", input.Sub)
		require.Len(t, input.MultiLevel, 5)
		require.Equal(t, []string{"b", "c", "d", "e", "f"}, input.MultiLevel)
	})

	t.Run("numeric subdomain", func(t *testing.T) {
		input, err := NewInput("123.example.com")
		require.NoError(t, err)
		require.Equal(t, "123", input.Sub)
	})

	t.Run("hyphenated subdomain", func(t *testing.T) {
		input, err := NewInput("api-v1.example.com")
		require.NoError(t, err)
		require.Equal(t, "api-v1", input.Sub)
	})

	t.Run("single character subdomain", func(t *testing.T) {
		input, err := NewInput("a.example.com")
		require.NoError(t, err)
		require.Equal(t, "a", input.Sub)
	})

	t.Run("url with scheme and port", func(t *testing.T) {
		input, err := NewInput("https://api.example.com:8443")
		require.NoError(t, err)
		require.Equal(t, "api", input.Sub)
		require.Equal(t, "example.com", input.Root)
	})

	t.Run("url with path", func(t *testing.T) {
		input, err := NewInput("https://api.example.com/path/to/resource")
		require.NoError(t, err)
		require.Equal(t, "api", input.Sub)
		require.Equal(t, "example.com", input.Root)
	})
}

func TestInputGetMap(t *testing.T) {
	t.Run("basic domain", func(t *testing.T) {
		input, err := NewInput("api.example.com")
		require.NoError(t, err)

		m := input.GetMap()
		require.Contains(t, m, "sub")
		require.Contains(t, m, "root")
		require.Contains(t, m, "sld")
		require.Contains(t, m, "tld")
		require.Equal(t, "api", m["sub"])
		require.Equal(t, "example.com", m["root"])
		require.Equal(t, "example", m["sld"])
		require.Equal(t, "com", m["tld"])
	})

	t.Run("multi-level subdomain", func(t *testing.T) {
		input, err := NewInput("api.v1.example.com")
		require.NoError(t, err)

		m := input.GetMap()
		require.Contains(t, m, "sub")
		require.Contains(t, m, "sub1")
		require.Equal(t, "api", m["sub"])
		require.Equal(t, "v1", m["sub1"])
	})

	t.Run("domain with eTLD", func(t *testing.T) {
		input, err := NewInput("api.example.co.uk")
		require.NoError(t, err)

		m := input.GetMap()
		require.Contains(t, m, "etld")
		require.Contains(t, m, "tld")
		require.Equal(t, "co.uk", m["etld"])
		require.Equal(t, "uk", m["tld"])
	})

	t.Run("root domain only", func(t *testing.T) {
		input, err := NewInput("example.com")
		require.NoError(t, err)

		m := input.GetMap()
		require.NotContains(t, m, "sub", "Empty sub should be purged")
		require.Contains(t, m, "root")
		require.Contains(t, m, "sld")
	})

	t.Run("deeply nested subdomains", func(t *testing.T) {
		input, err := NewInput("a.b.c.d.example.com")
		require.NoError(t, err)

		m := input.GetMap()
		require.Contains(t, m, "sub")
		require.Contains(t, m, "sub1")
		require.Contains(t, m, "sub2")
		require.Contains(t, m, "sub3")
		require.Equal(t, "a", m["sub"])
		require.Equal(t, "b", m["sub1"])
		require.Equal(t, "c", m["sub2"])
		require.Equal(t, "d", m["sub3"])
	})
}

func TestInputDifferentTLDs(t *testing.T) {
	testcases := []struct {
		domain       string
		expectedTLD  string
		expectedETLD string
		expectedSLD  string
	}{
		{"example.com", "com", "", "example"},
		{"example.co.uk", "uk", "co.uk", "example"},
		{"example.org", "org", "", "example"},
		{"example.com.au", "au", "com.au", "example"},
		{"example.ac.uk", "uk", "ac.uk", "example"},
	}

	for _, tc := range testcases {
		t.Run(tc.domain, func(t *testing.T) {
			input, err := NewInput(tc.domain)
			require.NoError(t, err)
			require.Equal(t, tc.expectedTLD, input.TLD)
			require.Equal(t, tc.expectedETLD, input.ETLD)
			require.Equal(t, tc.expectedSLD, input.SLD)
		})
	}
}

func BenchmarkNewInput(b *testing.B) {
	domains := []string{
		"api.example.com",
		"api.v1.example.com",
		"deep.nested.subdomain.example.co.uk",
		"https://api.example.com:8443/path",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, d := range domains {
			_, _ = NewInput(d)
		}
	}
}

func BenchmarkInputGetMap(b *testing.B) {
	input, _ := NewInput("api.v1.v2.example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = input.GetMap()
	}
}
