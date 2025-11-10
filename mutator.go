package alterx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/projectdiscovery/fasttemplate"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/utils/dedupe"
	errorutil "github.com/projectdiscovery/utils/errors"
	sliceutil "github.com/projectdiscovery/utils/slice"
)

var (
	extractNumbers   = regexp.MustCompile(`[0-9]+`)
	extractWords     = regexp.MustCompile(`[a-zA-Z0-9]+`)
	extractWordsOnly = regexp.MustCompile(`[a-zA-Z]{3,}`)
)

// Options contains configuration for the Mutator
type Options struct {
	// Domains is the list of domains to use as base for permutations
	Domains []string
	// Payloads contains words to use while creating permutations
	// If empty, DefaultWordList is used
	Payloads map[string][]string
	// Patterns is the list of patterns to use while creating permutations
	// If empty, DefaultPatterns are used
	Patterns []string
	// Limit restricts output results (0 = no limit)
	Limit int
	// Enrich when true, alterx extracts possible words from input
	// and adds them to default payloads word,number
	Enrich bool
	// MaxSize limits output data size in bytes
	MaxSize int
	// DedupeResults when true, deduplicates all results (default: true)
	DedupeResults bool
}

// Mutator
type Mutator struct {
	Options      *Options
	payloadCount int
	Inputs       []*Input // all processed inputs
	timeTaken    time.Duration
	// internal or unexported variables
	maxkeyLenInBytes int
}

// New creates and returns new mutator instance from options
func New(opts *Options) (*Mutator, error) {
	if len(opts.Domains) == 0 {
		return nil, fmt.Errorf("no domains provided: please provide at least one domain via -l flag or stdin")
	}

	// Set default for DedupeResults if not explicitly set
	if !opts.DedupeResults {
		opts.DedupeResults = true // Default to true
	}

	if len(opts.Payloads) == 0 {
		opts.Payloads = map[string][]string{}
		if len(DefaultConfig.Payloads) == 0 {
			return nil, fmt.Errorf("no payloads available: default payload configuration is empty and no custom payloads provided")
		}
		opts.Payloads = DefaultConfig.Payloads
	}
	if len(opts.Patterns) == 0 {
		if len(DefaultConfig.Patterns) == 0 {
			return nil, fmt.Errorf("no patterns available: default pattern configuration is empty and no custom patterns provided")
		}
		opts.Patterns = DefaultConfig.Patterns
	}
	// purge duplicates if any
	for k, v := range opts.Payloads {
		dedupe := sliceutil.Dedupe(v)
		if len(v) != len(dedupe) {
			gologger.Warning().Msgf("found %d duplicate payloads in '%s', removing duplicates", len(v)-len(dedupe), k)
			opts.Payloads[k] = dedupe
		}
	}
	m := &Mutator{
		Options: opts,
	}
	if err := m.validatePatterns(); err != nil {
		return nil, fmt.Errorf("pattern validation failed: %w", err)
	}
	if err := m.prepareInputs(); err != nil {
		return nil, err
	}
	if opts.Enrich {
		m.enrichPayloads()
	}
	return m, nil
}

// Execute calculates all permutations using input wordlist and patterns
// and writes them to a string channel. The context can be used to cancel
// the operation. Results are returned via a read-only channel.
func (m *Mutator) Execute(ctx context.Context) <-chan string {
	var maxBytes int
	if m.Options.DedupeResults {
		count := m.EstimateCount()
		maxBytes = count * m.maxkeyLenInBytes
	}

	results := make(chan string, len(m.Options.Patterns))
	go func() {
		defer close(results)
		now := time.Now()

		for _, v := range m.Inputs {
			// Check for cancellation at the input level
			select {
			case <-ctx.Done():
				m.timeTaken = time.Since(now)
				return
			default:
			}

			varMap := getSampleMap(v.GetMap(), m.Options.Payloads)
			for _, pattern := range m.Options.Patterns {
				// Check for cancellation at the pattern level
				select {
				case <-ctx.Done():
					m.timeTaken = time.Since(now)
					return
				default:
				}

				if err := checkMissing(pattern, varMap); err == nil {
					statement := Replace(pattern, v.GetMap())
					m.clusterBomb(ctx, statement, results)
				} else {
					gologger.Warning().Msgf("pattern '%s' has missing variables: %v, skipping", pattern, err)
				}
			}
		}
		m.timeTaken = time.Since(now)
	}()

	if m.Options.DedupeResults {
		// drain results
		d := dedupe.NewDedupe(results, maxBytes)
		d.Drain()
		return d.GetResults()
	}
	return results
}

// ExecuteWithWriter executes Mutator and writes results directly to a type that implements io.Writer interface.
// The context can be used to cancel the operation.
func (m *Mutator) ExecuteWithWriter(ctx context.Context, writer io.Writer) error {
	if writer == nil {
		return errorutil.NewWithTag("alterx", "writer destination cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	resChan := m.Execute(ctx)
	m.payloadCount = 0
	remainingSize := m.Options.MaxSize

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case value, ok := <-resChan:
			if !ok {
				gologger.Info().Msgf("Generated %d permutations in %s", m.payloadCount, m.Time())
				return nil
			}

			// Skip if limit reached
			if m.Options.Limit > 0 && m.payloadCount >= m.Options.Limit {
				continue
			}

			// Skip if max size reached
			if m.Options.MaxSize > 0 && remainingSize <= 0 {
				continue
			}

			// Skip domains starting with hyphen (invalid)
			if strings.HasPrefix(value, "-") {
				continue
			}

			outputData := []byte(value + "\n")

			// Check if writing this would exceed size limit
			if m.Options.MaxSize > 0 && len(outputData) > remainingSize {
				remainingSize = 0
				continue
			}

			n, err := writer.Write(outputData)
			if err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			// Update remaining size limit after each write
			if m.Options.MaxSize > 0 {
				remainingSize -= n
			}
			m.payloadCount++
		}
	}
}

// ExecuteWithWriterLegacy is the legacy version of ExecuteWithWriter that uses context.Background()
// Deprecated: Use ExecuteWithWriter with explicit context instead
func (m *Mutator) ExecuteWithWriterLegacy(writer io.Writer) error {
	return m.ExecuteWithWriter(context.Background(), writer)
}

// EstimateCount estimates number of payloads that will be created
// without actually executing/creating permutations
func (m *Mutator) EstimateCount() int {
	counter := 0
	for _, v := range m.Inputs {
		varMap := getSampleMap(v.GetMap(), m.Options.Payloads)
		for _, pattern := range m.Options.Patterns {
			if err := checkMissing(pattern, varMap); err == nil {
				// if say patterns is {{sub}}.{{sub1}}-{{word}}.{{root}}
				// and input domain is api.scanme.sh its clear that {{sub1}} here will be empty/missing
				// in such cases `alterx` silently skips that pattern for that specific input
				// this way user can have a long list of patterns but they are only used if all required data is given (much like self-contained templates)
				statement := Replace(pattern, v.GetMap())
				bin := unsafeToBytes(statement)
				if m.maxkeyLenInBytes < len(bin) {
					m.maxkeyLenInBytes = len(bin)
				}
				varsUsed := getAllVars(statement)
				if len(varsUsed) == 0 {
					counter += 1
				} else {
					tmpCounter := 1
					for _, word := range varsUsed {
						tmpCounter *= len(m.Options.Payloads[word])
					}
					counter += tmpCounter
				}
			}
		}
	}
	return counter
}

// DryRun executes payloads without storing and returns number of payloads created
// this value is also stored in variable and can be accessed via getter `PayloadCount`
func (m *Mutator) DryRun() int {
	m.payloadCount = 0
	err := m.ExecuteWithWriter(context.Background(), io.Discard)
	if err != nil {
		gologger.Error().Msgf("alterx dry run failed: %v", err)
	}
	return m.payloadCount
}

// clusterBomb calculates all payloads of clusterbomb attack and sends them to result channel
// It respects context cancellation to allow early termination
func (m *Mutator) clusterBomb(ctx context.Context, template string, results chan string) {
	// Early Exit: this is what saves clusterBomb from stackoverflows and reduces
	// n*len(n) iterations and n recursions
	varsUsed := getAllVars(template)
	if len(varsUsed) == 0 {
		// clusterBomb is not required
		// just send existing template as result and exit
		select {
		case results <- template:
		case <-ctx.Done():
		}
		return
	}
	payloadSet := map[string][]string{}
	// instead of sending all payloads only send payloads that are used
	// in template/statement
	leftmostPart, _, _ := strings.Cut(template, ".")
	for _, v := range varsUsed {
		payloadSet[v] = []string{}
		for _, word := range m.Options.Payloads[v] {
			if !strings.HasPrefix(leftmostPart, word) && !strings.HasSuffix(leftmostPart, word) {
				// skip all words that are already present in leftmost part, it is highly unlikely
				// we will ever find api-api.example.com
				payloadSet[v] = append(payloadSet[v], word)
			}
		}
	}
	payloads := NewIndexMap(payloadSet)
	// in clusterBomb attack no of payloads generated are
	// len(first_set)*len(second_set)*len(third_set)....
	callbackFunc := func(varMap map[string]interface{}) bool {
		select {
		case results <- Replace(template, varMap):
			return true
		case <-ctx.Done():
			return false
		}
	}
	ClusterBomb(payloads, callbackFunc, []string{})
}

// prepareInputs processes and validates all input domains
func (m *Mutator) prepareInputs() error {
	var errors []string
	var allInputs []*Input

	for _, domain := range m.Options.Domains {
		input, err := NewInput(domain)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", domain, err))
			continue
		}
		allInputs = append(allInputs, input)
	}

	m.Inputs = allInputs

	// If ALL inputs failed, return error
	if len(allInputs) == 0 {
		if len(errors) > 0 {
			return fmt.Errorf("all %d input domains failed to parse: %s", len(m.Options.Domains), strings.Join(errors, "; "))
		}
		return fmt.Errorf("no valid inputs were processed from %d provided domains", len(m.Options.Domains))
	}

	// If some inputs failed, log warnings
	if len(errors) > 0 {
		gologger.Warning().Msgf("failed to parse %d/%d domains: %s", len(errors), len(m.Options.Domains), strings.Join(errors, "; "))
	}

	return nil
}

// validates all patterns by compiling them
func (m *Mutator) validatePatterns() error {
	for _, v := range m.Options.Patterns {
		// check if all placeholders are correctly used and are valid
		if _, err := fasttemplate.NewTemplate(v, ParenthesisOpen, ParenthesisClose); err != nil {
			return err
		}
	}
	return nil
}

// enrichPayloads extract possible words and adds them to default wordlist
func (m *Mutator) enrichPayloads() {
	var temp bytes.Buffer
	for _, v := range m.Inputs {
		temp.WriteString(v.Sub + " ")
		if len(v.MultiLevel) > 0 {
			temp.WriteString(strings.Join(v.MultiLevel, " "))
		}
	}
	numbers := extractNumbers.FindAllString(temp.String(), -1)
	extraWords := extractWords.FindAllString(temp.String(), -1)
	extraWordsOnly := extractWordsOnly.FindAllString(temp.String(), -1)
	if len(extraWordsOnly) > 0 {
		extraWords = append(extraWords, extraWordsOnly...)
		extraWords = sliceutil.Dedupe(extraWords)
	}

	if len(m.Options.Payloads["word"]) > 0 {
		extraWords = append(extraWords, m.Options.Payloads["word"]...)
		m.Options.Payloads["word"] = sliceutil.Dedupe(extraWords)
	}
	if len(m.Options.Payloads["number"]) > 0 {
		numbers = append(numbers, m.Options.Payloads["number"]...)
		m.Options.Payloads["number"] = sliceutil.Dedupe(numbers)
	}
}

// PayloadCount returns total estimated payloads count
func (m *Mutator) PayloadCount() int {
	if m.payloadCount == 0 {
		return m.EstimateCount()
	}
	return m.payloadCount
}

// Time returns time taken to create permutations in seconds
func (m *Mutator) Time() string {
	return fmt.Sprintf("%.4fs", m.timeTaken.Seconds())
}
