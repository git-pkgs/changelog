// Package changelog parses changelog files into structured entries.
//
// It supports Keep a Changelog (## [version] - date), markdown headers
// (## version or ### version), setext/underline style (version\n=====), and
// common single-line version headers. Format detection is automatic by default.
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

	"github.com/git-pkgs/vers"
)

// Format represents a changelog file format.
type Format int

const (
	FormatAuto           Format = iota // Auto-detect format
	FormatKeepAChangelog               // ## [version] - date
	FormatMarkdown                     // ## version (date)
	FormatUnderline                    // version\n=====
)

// Entry holds the parsed data for a single changelog version.
type Entry struct {
	Date    *time.Time
	Content string
}

const versionTokenPattern = `[\w.+-]+\.[\w.+-]*[a-zA-Z0-9]`

// Compiled patterns for each format.
var (
	keepAChangelog  = regexp.MustCompile(`(?m)^##\s+\[([^\]]+)\](?:\s+-\s+(\d{4}-\d{2}-\d{2}))?`)
	markdownHeader  = regexp.MustCompile(`(?m)^#{1,3}\s+v?(` + versionTokenPattern + `)(?:\s+\((\d{4}-\d{2}-\d{2})\))?`)
	underlineHeader = regexp.MustCompile(`(?m)^(` + versionTokenPattern + `)\n[=-]+`)
	bulletHeader    = regexp.MustCompile(`(?mi)^[+*\-][ \t]+(?:version[ \t]+)?(` + versionTokenPattern + `)(?:[ \t]+\((\d{4}-\d{2}-\d{2})\))?[ \t]*$`)
	colonHeader     = regexp.MustCompile(`(?m)^(` + versionTokenPattern + `):(?:[ \t]+.*)?$`)
	bracketHeader   = regexp.MustCompile(`(?m)^\[(` + versionTokenPattern + `)\](?:[ \t]+.*)?$`)
	adornedHeader   = regexp.MustCompile(`(?m)^(?:!+|==+)[ \t]+(` + versionTokenPattern + `)(?:[ \t]+.*)?$`)
	dateHeader      = regexp.MustCompile(`(?mi)^\d{4}-\d{2}-\d{2}(?:[ \t]+[-:])?[ \t]+(?:version[ \t]+)?(` + versionTokenPattern + `)(?:[ \t]+.*)?$`)
	bareHeader      = regexp.MustCompile(`(?m)^(` + versionTokenPattern + `)(?:[ \t]+.*)?$`)
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
	start   int
	line    int
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

// VersionsBetween returns valid version strings greater than from and less
// than or equal to to, newest first. An empty bound leaves that side open.
func (p *Parser) VersionsBetween(from, to string) []string {
	p.ensureParsed()
	from = trimVersionPrefix(from)
	to = trimVersionPrefix(to)
	versions := make([]string, 0, len(p.entries))
	for _, ve := range p.entries {
		version := trimVersionPrefix(ve.version)
		if !vers.Valid(version) {
			continue
		}
		if from != "" && vers.Compare(version, from) <= 0 {
			continue
		}
		if to != "" && vers.Compare(version, to) > 0 {
			continue
		}
		versions = append(versions, ve.version)
	}

	slices.SortStableFunc(versions, func(a, b string) int {
		return vers.Compare(trimVersionPrefix(b), trimVersionPrefix(a))
	})
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
// Non-empty bounds must match versions extracted with the selected format.
// A leading "v" or "V" prefix is ignored when matching bounds.
// Returns the content and true if found, or empty string and false if not.
func (p *Parser) Between(oldVersion, newVersion string) (string, bool) {
	p.ensureParsed()
	oldIndex := p.indexForVersion(oldVersion)
	newIndex := p.indexForVersion(newVersion)
	if oldVersion != "" && oldIndex < 0 {
		return "", false
	}
	if newVersion != "" && newIndex < 0 {
		return "", false
	}

	var start, end int

	switch {
	case oldIndex >= 0 && newIndex >= 0:
		if oldIndex < newIndex {
			// Ascending: exclude old and include new
			start = p.entries[oldIndex+1].start
			end = p.contentEnd(newIndex)
		} else {
			// Descending (typical): new appears first, take from new to old
			start = p.entries[newIndex].start
			end = p.entries[oldIndex].start
		}
	case oldIndex >= 0:
		if p.entries[oldIndex].line == 0 {
			return "", false
		}
		start = 0
		end = p.entries[oldIndex].start
	case newIndex >= 0:
		start = p.entries[newIndex].start
		end = len(p.content)
	default:
		return "", false
	}

	result := p.content[start:end]
	result = strings.TrimRight(result, " \t\n")
	return result, true
}

func (p *Parser) contentEnd(index int) int {
	if index+1 < len(p.entries) {
		return p.entries[index+1].start
	}
	return len(p.content)
}

// LineForVersion returns the 0-based line number for an extracted version
// header, or -1 if not found. Strips a leading "v" prefix for matching.
func (p *Parser) LineForVersion(version string) int {
	p.ensureParsed()
	index := p.indexForVersion(version)
	if index < 0 {
		return -1
	}
	return p.entries[index].line
}

func trimVersionPrefix(version string) string {
	version = strings.TrimPrefix(version, "v")
	return strings.TrimPrefix(version, "V")
}

func (p *Parser) detectFormat() *regexp.Regexp {
	patterns := []*regexp.Regexp{
		keepAChangelog,
		underlineHeader,
		markdownHeader,
		bulletHeader,
		colonHeader,
		bracketHeader,
		adornedHeader,
		dateHeader,
		bareHeader,
	}
	selected := markdownHeader
	matchCount := 0
	// Declaration order breaks ties between formats with the same header count.
	for _, pattern := range patterns {
		matches := len(pattern.FindAllStringIndex(p.content, -1))
		if matches > matchCount {
			selected = pattern
			matchCount = matches
		}
	}
	return selected
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

	line := 0
	previousMatch := 0
	for i, match := range matches {
		line += strings.Count(p.content[previousMatch:match[0]], "\n")
		previousMatch = match[0]
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
			start:   match[0],
			line:    line,
			entry: Entry{
				Date:    datep,
				Content: content,
			},
		})
	}
}

func (p *Parser) indexForVersion(version string) int {
	if version == "" {
		return -1
	}
	target := trimVersionPrefix(version)
	for i, entry := range p.entries {
		if trimVersionPrefix(entry.version) == target {
			return i
		}
	}
	return -1
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
