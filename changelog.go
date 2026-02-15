// Package changelog parses changelog files into structured entries.
//
// It supports three common formats: Keep a Changelog (## [version] - date),
// markdown headers (## version or ### version), and setext/underline style
// (version\n=====). Format detection is automatic by default.
//
// Basic usage:
//
//	p := changelog.Parse(content)
//	for _, v := range p.Versions() {
//	    entry, _ := p.Entry(v)
//	    fmt.Printf("%s: %s\n", v, entry.Content)
//	}
//
// Parse a file:
//
//	p, err := changelog.ParseFile("CHANGELOG.md")
//
// Find and parse a changelog in a directory:
//
//	p, err := changelog.FindAndParse(".")
package changelog

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Format represents a changelog file format.
type Format int

const (
	FormatAuto          Format = iota // Auto-detect format
	FormatKeepAChangelog              // ## [version] - date
	FormatMarkdown                    // ## version (date)
	FormatUnderline                   // version\n=====
)

// Entry holds the parsed data for a single changelog version.
type Entry struct {
	Date    *time.Time
	Content string
}

// Compiled patterns for each format.
var (
	keepAChangelog = regexp.MustCompile(`(?m)^##\s+\[([^\]]+)\](?:\s+-\s+(\d{4}-\d{2}-\d{2}))?`)
	markdownHeader = regexp.MustCompile(`(?m)^#{1,3}\s+v?([\w.+-]+\.[\w.+-]+[a-zA-Z0-9])(?:\s+\((\d{4}-\d{2}-\d{2})\))?`)
	underlineHeader = regexp.MustCompile(`(?m)^([\w.+-]+\.[\w.+-]+[a-zA-Z0-9])\n[=-]+`)
)

// Common changelog filenames in priority order.
var changelogFilenames = []string{
	"changelog",
	"news",
	"changes",
	"history",
	"release",
	"whatsnew",
	"releases",
}

// Allowed changelog file extensions.
var changelogExtensions = []string{".md", ".txt", ".rst", ".rdoc", ".markdown", ""}

type versionEntry struct {
	version string
	entry   Entry
}

// Parser holds the parsed changelog data and provides access methods.
type Parser struct {
	content    string
	pattern    *regexp.Regexp
	matchGroup int
	entries    []versionEntry
	parsed     bool
}

// Parse creates a parser with automatic format detection.
func Parse(content string) *Parser {
	p := &Parser{
		content:    content,
		matchGroup: 1,
	}
	p.pattern = p.detectFormat()
	return p
}

// ParseWithFormat creates a parser using the specified format.
func ParseWithFormat(content string, format Format) *Parser {
	p := &Parser{
		content:    content,
		matchGroup: 1,
	}
	switch format {
	case FormatKeepAChangelog:
		p.pattern = keepAChangelog
	case FormatMarkdown:
		p.pattern = markdownHeader
	case FormatUnderline:
		p.pattern = underlineHeader
	default:
		p.pattern = p.detectFormat()
	}
	return p
}

// ParseWithPattern creates a parser using a custom regex pattern.
// The pattern must have at least one capture group for the version string.
// An optional second capture group captures the date (YYYY-MM-DD).
// The (?m) flag is automatically added if not already present, so that
// ^ and $ match line boundaries.
func ParseWithPattern(content string, pattern *regexp.Regexp) *Parser {
	expr := pattern.String()
	if !strings.Contains(expr, "(?m)") {
		pattern = regexp.MustCompile("(?m)" + expr)
	}
	return &Parser{
		content:    content,
		pattern:    pattern,
		matchGroup: 1,
	}
}

// ParseFile reads and parses a changelog file.
func ParseFile(path string) (*Parser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data)), nil
}

// FindChangelog locates a changelog file in the given directory.
// Returns the path to the changelog file, or empty string if not found.
func FindChangelog(directory string) (string, error) {
	dirEntries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}

	var files []string
	for _, e := range dirEntries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}

	for _, name := range changelogFilenames {
		var candidates []string
		for _, f := range files {
			if strings.HasSuffix(strings.ToLower(f), ".sh") {
				continue
			}
			lower := strings.ToLower(f)
			base := lower
			ext := filepath.Ext(lower)
			if ext != "" {
				base = lower[:len(lower)-len(ext)]
			}
			if base != name {
				continue
			}
			if slices.Contains(changelogExtensions, ext) {
				candidates = append(candidates, f)
			}
		}

		if len(candidates) == 1 {
			return filepath.Join(directory, candidates[0]), nil
		}

		for _, candidate := range candidates {
			path := filepath.Join(directory, candidate)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			size := info.Size()
			if size > 1_000_000 || size < 100 {
				continue
			}
			return path, nil
		}
	}

	return "", nil
}

// FindAndParse locates a changelog file in the directory and parses it.
func FindAndParse(directory string) (*Parser, error) {
	path, err := FindChangelog(directory)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return ParseFile(path)
}

// Versions returns the version strings in the order they appear in the changelog.
func (p *Parser) Versions() []string {
	p.ensureParsed()
	versions := make([]string, len(p.entries))
	for i, ve := range p.entries {
		versions[i] = ve.version
	}
	return versions
}

// Entry returns the entry for a specific version.
func (p *Parser) Entry(version string) (Entry, bool) {
	p.ensureParsed()
	for _, ve := range p.entries {
		if ve.version == version {
			return ve.entry, true
		}
	}
	return Entry{}, false
}

// Entries returns all entries as a map. Note that Go maps do not preserve
// insertion order; use Versions() + Entry() if order matters.
func (p *Parser) Entries() map[string]Entry {
	p.ensureParsed()
	m := make(map[string]Entry, len(p.entries))
	for _, ve := range p.entries {
		m[ve.version] = ve.entry
	}
	return m
}

// Between returns the content between two version headers.
// Either version can be empty to indicate the start or end of the changelog.
// Returns the content and true if found, or empty string and false if not.
func (p *Parser) Between(oldVersion, newVersion string) (string, bool) {
	oldLine := p.LineForVersion(oldVersion)
	newLine := p.LineForVersion(newVersion)
	lines := strings.Split(p.content, "\n")

	var start, end int
	found := false

	if oldLine >= 0 && newLine >= 0 {
		if oldLine < newLine {
			// Ascending: old appears first, take from old line to end
			start = oldLine
			end = len(lines)
		} else {
			// Descending (typical): new appears first, take from new to old
			start = newLine
			end = oldLine
		}
		found = true
	} else if oldLine >= 0 {
		if oldLine == 0 {
			return "", false
		}
		start = 0
		end = oldLine
		found = true
	} else if newLine >= 0 {
		start = newLine
		end = len(lines)
		found = true
	}

	if !found {
		return "", false
	}

	result := strings.Join(lines[start:end], "\n")
	result = strings.TrimRight(result, " \t\n")
	return result, true
}

// LineForVersion returns the 0-based line number where the given version
// header appears, or -1 if not found. Strips a leading "v" prefix for matching.
func (p *Parser) LineForVersion(version string) int {
	if version == "" {
		return -1
	}

	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	escaped := regexp.QuoteMeta(version)

	// Go's regexp doesn't support lookbehinds, so we check surrounding
	// characters manually after finding a match.
	versionRe := regexp.MustCompile(escaped)
	rangeRe := regexp.MustCompile(escaped + `\.\.`)

	lines := strings.Split(p.content, "\n")

	for i, line := range lines {
		if !containsVersion(line, versionRe) {
			continue
		}
		if rangeRe.MatchString(line) {
			continue
		}

		// Check if this line looks like a version header
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "==") {
			return i
		}
		versionLineRe := regexp.MustCompile(`^v?` + escaped + `:?\s`)
		if versionLineRe.MatchString(line) {
			return i
		}
		bracketRe := regexp.MustCompile(`^\[` + escaped + `\]`)
		if bracketRe.MatchString(line) {
			return i
		}
		bulletRe := regexp.MustCompile(`(?i)^[+*\-]\s+(version\s+)?` + escaped)
		if bulletRe.MatchString(line) {
			return i
		}
		dateLineRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
		if dateLineRe.MatchString(line) {
			return i
		}
		// Check if next line is an underline
		if i+1 < len(lines) {
			underlineRe := regexp.MustCompile(`^[=\-+]{3,}\s*$`)
			if underlineRe.MatchString(lines[i+1]) {
				return i
			}
		}
	}

	return -1
}

// containsVersion checks if a line contains the version string without it
// being a substring of a longer version (e.g. 1.0.1 should not match inside 1.0.10).
// Allows a preceding 'v' or 'V' since version headers commonly use that prefix.
func containsVersion(line string, versionRe *regexp.Regexp) bool {
	for _, loc := range versionRe.FindAllStringIndex(line, -1) {
		// Check char before match: must not be a dot or word char (except v/V prefix)
		if loc[0] > 0 {
			prev := line[loc[0]-1]
			if prev == '.' {
				continue
			}
			if isWordChar(prev) && prev != 'v' && prev != 'V' {
				continue
			}
		}
		// Check char after match: must not be dot, dash, or word char
		if loc[1] < len(line) {
			next := line[loc[1]]
			if next == '.' || next == '-' || isWordChar(next) {
				continue
			}
		}
		return true
	}
	return false
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func (p *Parser) detectFormat() *regexp.Regexp {
	if keepAChangelog.MatchString(p.content) {
		return keepAChangelog
	}
	if underlineHeader.MatchString(p.content) {
		return underlineHeader
	}
	return markdownHeader
}

func (p *Parser) ensureParsed() {
	if p.parsed {
		return
	}
	p.parsed = true
	p.doParse()
}

func (p *Parser) doParse() {
	if p.content == "" {
		return
	}

	matches := p.pattern.FindAllStringSubmatchIndex(p.content, -1)
	if matches == nil {
		return
	}

	for i, match := range matches {
		version := p.extractGroup(match, p.matchGroup)
		date := p.extractDate(match)

		headerEnd := match[1] // end of entire match
		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0] // start of next match
		} else {
			contentEnd = len(p.content)
		}

		content := strings.TrimSpace(p.content[headerEnd:contentEnd])

		var datep *time.Time
		if date != nil {
			datep = date
		}

		p.entries = append(p.entries, versionEntry{
			version: version,
			entry: Entry{
				Date:    datep,
				Content: content,
			},
		})
	}
}

func (p *Parser) extractGroup(match []int, group int) string {
	start := match[group*2]
	end := match[group*2+1]
	if start < 0 {
		return ""
	}
	return p.content[start:end]
}

func (p *Parser) extractDate(match []int) *time.Time {
	group := p.matchGroup + 1
	if group*2+1 >= len(match) {
		return nil
	}
	start := match[group*2]
	end := match[group*2+1]
	if start < 0 {
		return nil
	}
	dateStr := p.content[start:end]
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil
	}
	return &t
}
