package alterx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/projectdiscovery/gologger"
	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/net/publicsuffix"
)

// Input contains parsed/evaluated data of a URL
type Input struct {
	TLD        string   // only TLD (right most part of subdomain) ex: `.uk`
	ETLD       string   // Simply put public suffix (ex: co.uk)
	SLD        string   // Second-level domain (ex: scanme)
	Root       string   // Root Domain (eTLD+1) of Subdomain
	Sub        string   // Sub or LeftMost prefix of subdomain
	Suffix     string   // suffix is everything except `Sub` (Note: if domain is not multilevel Suffix==Root)
	MultiLevel []string // (Optional) store prefix of multi level subdomains
}

// GetMap returns variables map of input
func (i *Input) GetMap() map[string]interface{} {
	m := map[string]interface{}{
		"tld":    i.TLD,
		"etld":   i.ETLD,
		"sld":    i.SLD,
		"root":   i.Root,
		"sub":    i.Sub,
		"suffix": i.Suffix,
	}
	for k, v := range i.MultiLevel {
		m["sub"+strconv.Itoa(k+1)] = v
	}
	for k, v := range m {
		if v == "" {
			// purge empty vars
			delete(m, k)
		}
	}
	return m
}

// NewInput parses a URL or domain string into structured Input variables.
// It extracts TLD, eTLD, SLD, root domain, subdomains, and multi-level components.
func NewInput(inputURL string) (*Input, error) {
	URL, err := urlutil.Parse(inputURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	hostname := URL.Hostname()
	if hostname == "" {
		return nil, fmt.Errorf("empty hostname in URL")
	}

	// Handle wildcard domains
	if strings.Contains(hostname, "*") {
		if strings.HasPrefix(hostname, "*.") {
			// Remove leading wildcard (e.g., *.example.com -> example.com)
			hostname = strings.TrimPrefix(hostname, "*.")
			URL.Host = strings.Replace(URL.Host, URL.Hostname(), hostname, 1)
		}
		// If * is present in middle (e.g., prod.*.hackerone.com), reject it
		if strings.Contains(hostname, "*") {
			return nil, fmt.Errorf("wildcard in middle of domain not supported: %s", inputURL)
		}
	}

	ivar := &Input{}

	// Extract public suffix (TLD or eTLD like .com or .co.uk)
	suffix, err := publicsuffix.PublicSuffix(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to extract public suffix: %w", err)
	}

	if strings.Contains(suffix, ".") {
		// Multi-part TLD like co.uk
		ivar.ETLD = suffix
		parts := strings.Split(suffix, ".")
		ivar.TLD = parts[len(parts)-1]
	} else {
		// Simple TLD like .com
		ivar.TLD = suffix
	}

	// Extract root domain (eTLD+1)
	rootDomain, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		// This happens if input is just a TLD (e.g., ".com" or "co.uk")
		return nil, fmt.Errorf("domain '%s' appears to be a public suffix without a registered domain", hostname)
	}

	ivar.Root = rootDomain

	// Extract second-level domain (SLD)
	if ivar.ETLD != "" {
		ivar.SLD = strings.TrimSuffix(rootDomain, "."+ivar.ETLD)
	} else {
		ivar.SLD = strings.TrimSuffix(rootDomain, "."+ivar.TLD)
	}

	// Extract subdomain components
	subdomainPrefix := strings.TrimSuffix(hostname, rootDomain)
	subdomainPrefix = strings.TrimSuffix(subdomainPrefix, ".")

	if strings.Contains(subdomainPrefix, ".") {
		// Multi-level subdomain (e.g., api.v1.example.com -> sub=api, sub1=v1)
		prefixes := strings.Split(subdomainPrefix, ".")
		ivar.Sub = prefixes[0]
		ivar.MultiLevel = prefixes[1:]
	} else {
		ivar.Sub = subdomainPrefix
	}

	// Suffix is everything except the leftmost subdomain
	if ivar.Sub != "" {
		ivar.Suffix = strings.TrimPrefix(hostname, ivar.Sub+".")
	} else {
		ivar.Suffix = hostname
	}

	return ivar, nil
}
