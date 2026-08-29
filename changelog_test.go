package changelog

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func mustReadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseEmpty(t *testing.T) {
	p := Parse("")
	if len(p.Versions()) != 0 {
		t.Errorf("expected no versions, got %d", len(p.Versions()))
	}
	if len(p.Entries()) != 0 {
		t.Errorf("expected no entries, got %d", len(p.Entries()))
	}
}

func TestKeepAChangelogFormat(t *testing.T) {
	content := mustReadFixture(t, "keep_a_changelog.md")
	p := Parse(content)

	t.Run("detects format", func(t *testing.T) {
		if p.pattern != keepAChangelog {
			t.Error("expected keep-a-changelog pattern")
		}
	})

	t.Run("parses all versions", func(t *testing.T) {
		versions := p.Versions()
		if len(versions) != 4 {
			t.Fatalf("expected 4 versions, got %d", len(versions))
		}
		want := []string{"Unreleased", "1.1.0", "1.0.1", "1.0.0"}
		for i, v := range want {
			if versions[i] != v {
				t.Errorf("version[%d] = %q, want %q", i, versions[i], v)
			}
		}
	})

	t.Run("extracts dates", func(t *testing.T) {
		entry, ok := p.Entry("Unreleased")
		if !ok {
			t.Fatal("Unreleased not found")
		}
		if entry.Date != nil {
			t.Error("expected nil date for Unreleased")
		}

		entry, _ = p.Entry("1.1.0")
		assertDate(t, entry.Date, time.March, 15)

		entry, _ = p.Entry("1.0.1")
		assertDate(t, entry.Date, time.February, 1)

		entry, _ = p.Entry("1.0.0")
		assertDate(t, entry.Date, time.January, 15)
	})

	t.Run("extracts content", func(t *testing.T) {
		entry, _ := p.Entry("1.1.0")
		if !strings.Contains(entry.Content, "User authentication system") {
			t.Error("expected 1.1.0 content to contain 'User authentication system'")
		}
		if !strings.Contains(entry.Content, "Memory leak in connection pool") {
			t.Error("expected 1.1.0 content to contain 'Memory leak in connection pool'")
		}

		entry, _ = p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "Initial release") {
			t.Error("expected 1.0.0 content to contain 'Initial release'")
		}
	})
}

func TestMarkdownHeaderFormat(t *testing.T) {
	content := mustReadFixture(t, "markdown_header.md")
	p := ParseWithFormat(content, FormatMarkdown)

	t.Run("parses versions with dates", func(t *testing.T) {
		entry, ok := p.Entry("2.0.0")
		if !ok {
			t.Fatal("2.0.0 not found")
		}
		assertDate(t, entry.Date, time.March, 1)
	})

	t.Run("parses versions without dates", func(t *testing.T) {
		entry, ok := p.Entry("1.5.0")
		if !ok {
			t.Fatal("1.5.0 not found")
		}
		if entry.Date != nil {
			t.Error("expected nil date for 1.5.0")
		}
	})

	t.Run("parses h3 headers", func(t *testing.T) {
		_, ok := p.Entry("1.4.2")
		if !ok {
			t.Error("1.4.2 not found")
		}
	})

	t.Run("extracts content", func(t *testing.T) {
		entry, _ := p.Entry("2.0.0")
		if !strings.Contains(entry.Content, "Breaking changes") {
			t.Error("expected 2.0.0 content to contain 'Breaking changes'")
		}

		entry, _ = p.Entry("1.5.0")
		if !strings.Contains(entry.Content, "caching layer") {
			t.Error("expected 1.5.0 content to contain 'caching layer'")
		}
	})
}

func TestUnderlineFormat(t *testing.T) {
	content := mustReadFixture(t, "underline.md")
	p := ParseWithFormat(content, FormatUnderline)

	t.Run("parses equals underline", func(t *testing.T) {
		_, ok := p.Entry("3.0.0")
		if !ok {
			t.Error("3.0.0 not found")
		}
		_, ok = p.Entry("2.0.0")
		if !ok {
			t.Error("2.0.0 not found")
		}
	})

	t.Run("parses dash underline", func(t *testing.T) {
		_, ok := p.Entry("2.1.0")
		if !ok {
			t.Error("2.1.0 not found")
		}
	})

	t.Run("extracts content", func(t *testing.T) {
		entry, _ := p.Entry("3.0.0")
		if !strings.Contains(entry.Content, "Complete rewrite") {
			t.Error("expected 3.0.0 content to contain 'Complete rewrite'")
		}

		entry, _ = p.Entry("2.1.0")
		if !strings.Contains(entry.Content, "Bug fixes") {
			t.Error("expected 2.1.0 content to contain 'Bug fixes'")
		}
	})
}

func TestCustomPattern(t *testing.T) {
	t.Run("custom regex", func(t *testing.T) {
		content := "Version 1.2.0 released 2024-01-01\n- Feature A\n\nVersion 1.1.0 released 2023-12-01\n- Feature B\n"
		pattern := regexp.MustCompile(`^Version ([\d.]+) released (\d{4}-\d{2}-\d{2})`)
		p := ParseWithPattern(content, pattern)

		versions := p.Versions()
		if len(versions) != 2 {
			t.Fatalf("expected 2 versions, got %d", len(versions))
		}
		if versions[0] != "1.2.0" {
			t.Errorf("expected first version 1.2.0, got %s", versions[0])
		}
		if versions[1] != "1.1.0" {
			t.Errorf("expected second version 1.1.0, got %s", versions[1])
		}

		entry, _ := p.Entry("1.2.0")
		assertDate(t, entry.Date, time.January, 1)
	})

	t.Run("custom match group", func(t *testing.T) {
		content := "## Release v1.0.0\n\nContent here\n\n## Release v0.9.0\n\nMore content\n"
		pattern := regexp.MustCompile(`^## Release v([\d.]+)`)
		p := ParseWithPattern(content, pattern)

		versions := p.Versions()
		if len(versions) != 2 {
			t.Fatalf("expected 2 versions, got %d", len(versions))
		}
		if versions[0] != "1.0.0" {
			t.Errorf("expected 1.0.0, got %s", versions[0])
		}
		if versions[1] != "0.9.0" {
			t.Errorf("expected 0.9.0, got %s", versions[1])
		}
	})
}

func TestFormatDetection(t *testing.T) {
	t.Run("detects keep a changelog", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01\n\nContent")
		if p.pattern != keepAChangelog {
			t.Error("expected keep-a-changelog pattern")
		}
	})

	t.Run("detects underline", func(t *testing.T) {
		p := Parse("1.0.0\n=====\n\nContent")
		if p.pattern != underlineHeader {
			t.Error("expected underline pattern")
		}
	})

	t.Run("falls back to markdown", func(t *testing.T) {
		p := Parse("## 1.0.0\n\nContent")
		if p.pattern != markdownHeader {
			t.Error("expected markdown pattern")
		}
	})
}

func TestShortVersionHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		pattern *regexp.Regexp
		want    string
	}{
		{
			name:    "markdown",
			content: "## 1.0\n\nFirst\n\n## 1.1\n\nSecond\n\n## 1.2\n\nThird\n",
			pattern: markdownHeader,
			want:    "## 1.1\n\nSecond",
		},
		{
			name:    "underline",
			content: "1.0\n===\n\nFirst\n\n1.1\n===\n\nSecond\n\n1.2\n===\n\nThird\n",
			pattern: underlineHeader,
			want:    "1.1\n===\n\nSecond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.content)
			if p.pattern != tt.pattern {
				t.Fatalf("detected pattern %q, want %q", p.pattern, tt.pattern)
			}
			if !slices.Equal(p.Versions(), []string{"1.0", "1.1", "1.2"}) {
				t.Fatalf("Versions() = %v", p.Versions())
			}
			result, ok := p.Between("1.0", "1.1")
			if !ok {
				t.Fatal("expected result")
			}
			if result != tt.want {
				t.Errorf("Between() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestLineForVersion(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		version  string
		wantLine int
	}{
		{
			name:     "keep a changelog header",
			content:  "## [1.0.0] - 2024-01-01\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "v prefix in version arg",
			content:  "## v1.0.0\n\nContent",
			version:  "v1.0.0",
			wantLine: 0,
		},
		{
			name:     "strips v prefix for matching",
			content:  "## v1.0.0\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "underlined version",
			content:  "1.0.0\n=====\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "bullet point version",
			content:  "- version 1.0.0\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "colon version",
			content:  "1.0.0: Initial release\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "plain bracket version",
			content:  "[1.0.0]\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "bang version",
			content:  "! 1.0.0\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "equals version",
			content:  "== 1.0.0\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "date version",
			content:  "2024-01-01 - version 1.0.0\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "bare version",
			content:  "1.0.0 Initial release\n\nContent",
			version:  "1.0.0",
			wantLine: 0,
		},
		{
			name:     "not found",
			content:  "## [1.0.0] - 2024-01-01\n\nContent",
			version:  "2.0.0",
			wantLine: -1,
		},
		{
			name:     "empty version",
			content:  "## [1.0.0] - 2024-01-01\n\nContent",
			version:  "",
			wantLine: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.content)
			got := p.LineForVersion(tt.version)
			if got != tt.wantLine {
				t.Errorf("LineForVersion(%q) = %d, want %d", tt.version, got, tt.wantLine)
			}
		})
	}
}

func TestLineForVersionSubstring(t *testing.T) {
	content := "## [1.0.10] - 2024-02-01\n\nContent for 1.0.10\n\n## [1.0.1] - 2024-01-01\n\nContent for 1.0.1\n"
	p := Parse(content)

	if got := p.LineForVersion("1.0.10"); got != 0 {
		t.Errorf("LineForVersion(1.0.10) = %d, want 0", got)
	}
	if got := p.LineForVersion("1.0.1"); got != 4 {
		t.Errorf("LineForVersion(1.0.1) = %d, want 4", got)
	}
}

func TestLineForVersionRange(t *testing.T) {
	content := "Supports versions 1.0.0..2.0.0\n\n## [1.0.0]\n\nContent"
	p := Parse(content)

	if got := p.LineForVersion("1.0.0"); got != 2 {
		t.Errorf("LineForVersion(1.0.0) = %d, want 2", got)
	}
}

func TestVersionsBetween(t *testing.T) {
	content := "## [2.0.0]\n\nSecond\n\n## [Unreleased]\n\nNext\n\n## [1.0.0]\n\nFirst\n\n## [3.0.0]\n\nThird\n"
	p := Parse(content)

	tests := []struct {
		name string
		from string
		to   string
		want []string
	}{
		{name: "bounded", from: "1.0.0", to: "3.0.0", want: []string{"3.0.0", "2.0.0"}},
		{name: "open lower bound", to: "2.0.0", want: []string{"2.0.0", "1.0.0"}},
		{name: "open upper bound", from: "2.0.0", want: []string{"3.0.0"}},
		{name: "both bounds open", want: []string{"3.0.0", "2.0.0", "1.0.0"}},
		{name: "missing endpoints", from: "1.5.0", to: "2.5.0", want: []string{"2.0.0"}},
		{name: "uppercase v bounds", from: "V1.0.0", to: "V3.0.0", want: []string{"3.0.0", "2.0.0"}},
		{name: "equal endpoints", from: "2.0.0", to: "2.0.0", want: []string{}},
		{name: "inverted range", from: "3.0.0", to: "1.0.0", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.VersionsBetween(tt.from, tt.to)
			if !slices.Equal(got, tt.want) {
				t.Errorf("VersionsBetween(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestVersionsBetweenVersionFormats(t *testing.T) {
	t.Run("calver", func(t *testing.T) {
		p := Parse("## [2024.9]\n\nSeptember\n\n## [2024.10]\n\nOctober\n")
		got := p.VersionsBetween("2024.8", "2024.11")
		want := []string{"2024.10", "2024.9"}
		if !slices.Equal(got, want) {
			t.Errorf("VersionsBetween() = %v, want %v", got, want)
		}
	})

	t.Run("prerelease", func(t *testing.T) {
		p := Parse("## [1.0.0-beta.2]\n\nBeta 2\n\n## [1.0.0]\n\nStable\n\n## [1.0.0-beta.10]\n\nBeta 10\n")
		got := p.VersionsBetween("1.0.0-alpha", "1.0.0")
		want := []string{"1.0.0", "1.0.0-beta.10", "1.0.0-beta.2"}
		if !slices.Equal(got, want) {
			t.Errorf("VersionsBetween() = %v, want %v", got, want)
		}
	})

	t.Run("uppercase v headings", func(t *testing.T) {
		p := Parse("## [V1.0.0]\n\nFirst\n\n## [V2.0.0]\n\nSecond\n")
		got := p.VersionsBetween("V1.0.0", "V2.0.0")
		want := []string{"V2.0.0"}
		if !slices.Equal(got, want) {
			t.Errorf("VersionsBetween() = %v, want %v", got, want)
		}
	})

	t.Run("loose version", func(t *testing.T) {
		p := Parse("## [1.2b1]\n\nBeta 1\n\n## [1.2a1]\n\nAlpha 1\n")
		got := p.VersionsBetween("1.2a0", "1.2b1")
		want := []string{"1.2b1", "1.2a1"}
		if !slices.Equal(got, want) {
			t.Errorf("VersionsBetween() = %v, want %v", got, want)
		}
	})
}

func TestBetween(t *testing.T) {
	content := "## [3.0.0] - 2024-03-01\n\nVersion 3 content\n\n## [2.0.0] - 2024-02-01\n\nVersion 2 content\n\n## [1.0.0] - 2024-01-01\n\nVersion 1 content\n"
	p := Parse(content)

	t.Run("between two versions descending", func(t *testing.T) {
		result, ok := p.Between("1.0.0", "3.0.0")
		if !ok {
			t.Fatal("expected result")
		}
		if !strings.Contains(result, "Version 3 content") {
			t.Error("expected result to contain 'Version 3 content'")
		}
		if !strings.Contains(result, "Version 2 content") {
			t.Error("expected result to contain 'Version 2 content'")
		}
		if strings.Contains(result, "Version 1 content") {
			t.Error("expected result to NOT contain 'Version 1 content'")
		}
	})

	t.Run("from new version to end", func(t *testing.T) {
		result, ok := p.Between("", "2.0.0")
		if !ok {
			t.Fatal("expected result")
		}
		if !strings.Contains(result, "Version 2 content") {
			t.Error("expected result to contain 'Version 2 content'")
		}
		if !strings.Contains(result, "Version 1 content") {
			t.Error("expected result to contain 'Version 1 content'")
		}
	})

	t.Run("from start to old version", func(t *testing.T) {
		result, ok := p.Between("2.0.0", "")
		if !ok {
			t.Fatal("expected result")
		}
		if !strings.Contains(result, "Version 3 content") {
			t.Error("expected result to contain 'Version 3 content'")
		}
		if strings.Contains(result, "Version 2 content") {
			t.Error("expected result to NOT contain 'Version 2 content'")
		}
	})

	t.Run("neither found", func(t *testing.T) {
		_, ok := p.Between("9.0.0", "8.0.0")
		if ok {
			t.Error("expected no result when versions not found")
		}
	})

	t.Run("between adjacent versions ascending", func(t *testing.T) {
		ascending := "## [1.0.0]\n\nFirst\n\n## [2.0.0]\n\nSecond\n\n## [3.0.0]\n\nThird\n"
		ap := Parse(ascending)
		result, ok := ap.Between("1.0.0", "2.0.0")
		if !ok {
			t.Fatal("expected result")
		}
		want := "## [2.0.0]\n\nSecond"
		if result != want {
			t.Errorf("Between() = %q, want %q", result, want)
		}
	})

	t.Run("between non-adjacent versions ascending", func(t *testing.T) {
		ascending := "## [1.0.0]\n\nFirst\n\n## [2.0.0]\n\nSecond\n\n## [3.0.0]\n\nThird\n\n## [4.0.0]\n\nFourth\n"
		ap := Parse(ascending)
		result, ok := ap.Between("1.0.0", "3.0.0")
		if !ok {
			t.Fatal("expected result")
		}
		want := "## [2.0.0]\n\nSecond\n\n## [3.0.0]\n\nThird"
		if result != want {
			t.Errorf("Between() = %q, want %q", result, want)
		}
	})
}

func TestBetweenMissingBound(t *testing.T) {
	content := "## [3.0.0]\n\nThird\n\n## [2.0.0]\n\nSecond\n\n## [1.0.0]\n\nFirst\n"
	p := Parse(content)
	tests := []struct {
		name       string
		oldVersion string
		newVersion string
	}{
		{name: "old version", oldVersion: "9.0.0", newVersion: "2.0.0"},
		{name: "new version", oldVersion: "2.0.0", newVersion: "9.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := p.Between(tt.oldVersion, tt.newVersion); ok {
				t.Error("expected no result when a bound is not found")
			}
		})
	}
}

func TestBetweenVersionMentionInReleaseNote(t *testing.T) {
	tests := []struct {
		name string
		note string
	}{
		{name: "earlier version", note: "- version 0.9.0 is no longer supported"},
		{name: "later version", note: "- version 3.0.0 requires migration"},
		{name: "standalone later version", note: "- version 3.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ascending := "## [1.0.0]\n\nFirst\n\n## [2.0.0]\n\n" + tt.note + "\n\nSecond\n\n## [3.0.0]\n\nThird\n"
			result, ok := Parse(ascending).Between("1.0.0", "2.0.0")
			if !ok {
				t.Fatal("expected result")
			}
			want := "## [2.0.0]\n\n" + tt.note + "\n\nSecond"
			if result != want {
				t.Errorf("Between() = %q, want %q", result, want)
			}
		})
	}
}

func TestBetweenBuildMetadataAscending(t *testing.T) {
	ascending := "## [1.0.0+build.1]\n\nFirst\n\n## [1.0.0+build.2]\n\nSecond\n\n## [1.0.0+build.3]\n\nThird\n"
	result, ok := Parse(ascending).Between("1.0.0+build.1", "1.0.0+build.2")
	if !ok {
		t.Fatal("expected result")
	}
	want := "## [1.0.0+build.2]\n\nSecond"
	if result != want {
		t.Errorf("Between() = %q, want %q", result, want)
	}
}

func TestBetweenRejectsMixedHeaders(t *testing.T) {
	content := "1.0.0: First\n\n## [2.0.0]\n\nSecond\n\n## [3.0.0]\n\nThird\n\n4.0.0: Fourth\n"
	if _, ok := Parse(content).Between("1.0.0", "2.0.0"); ok {
		t.Error("expected no result when a bound uses a different header format")
	}
}

func TestBetweenCustomPatternAscending(t *testing.T) {
	content := "Version 1.0.0\n\nFirst\n\nVersion 2.0.0\n\nSecond\n\nVersion 3.0.0\n\nThird\n"
	pattern := regexp.MustCompile(`^Version ([\d.]+)`)
	result, ok := ParseWithPattern(content, pattern).Between("1.0.0", "2.0.0")
	if !ok {
		t.Fatal("expected result")
	}
	want := "Version 2.0.0\n\nSecond"
	if result != want {
		t.Errorf("Between() = %q, want %q", result, want)
	}
}

func TestBetweenAlternateHeadersAscending(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		oldVersion string
		newVersion string
		want       string
		pattern    *regexp.Regexp
	}{
		{
			name:       "bullet headers",
			content:    "- version 1.0.0\n\nFirst\n\n- version 2.0.0\n\nSecond\n\n- version 3.0.0\n\nThird\n\n- version 4.0.0\n\nFourth\n",
			oldVersion: "1.0.0",
			newVersion: "3.0.0",
			want:       "- version 2.0.0\n\nSecond\n\n- version 3.0.0\n\nThird",
			pattern:    bulletHeader,
		},
		{
			name:       "bullet headers with version bullet",
			content:    "- version 1.0.0\n\nFirst\n\n- version 2.0.0\n\n- version 0.9.0 is no longer supported\n\nSecond\n\n- version 3.0.0\n\nThird\n",
			oldVersion: "1.0.0",
			newVersion: "2.0.0",
			want:       "- version 2.0.0\n\n- version 0.9.0 is no longer supported\n\nSecond",
			pattern:    bulletHeader,
		},
		{
			name:       "bullet headers with suffix and version bullet",
			content:    "- version 1.0.0 (2024-01-01)\n\nFirst\n\n- version 2.0.0 (2024-02-01)\n\n- version 0.9.0 is no longer supported\n\nSecond\n\n- version 3.0.0 (2024-03-01)\n\nThird\n",
			oldVersion: "1.0.0",
			newVersion: "2.0.0",
			want:       "- version 2.0.0 (2024-02-01)\n\n- version 0.9.0 is no longer supported\n\nSecond",
			pattern:    bulletHeader,
		},
		{
			name:       "colon headers",
			content:    "2024.9: September\n\n2024.10: October\n\n2024.11: November\n\n2024.12: December\n",
			oldVersion: "2024.9",
			newVersion: "2024.11",
			want:       "2024.10: October\n\n2024.11: November",
			pattern:    colonHeader,
		},
		{
			name:       "colon headers with version bullet",
			content:    "2024.9: September\n\n2024.10: October\n\n- version 0.9.0\n\nMore\n\n2024.11: November\n",
			oldVersion: "2024.9",
			newVersion: "2024.10",
			want:       "2024.10: October\n\n- version 0.9.0\n\nMore",
			pattern:    colonHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.content)
			if p.pattern != tt.pattern {
				t.Fatalf("detected pattern %q, want %q", p.pattern, tt.pattern)
			}
			if p.LineForVersion(tt.newVersion) < 0 {
				t.Fatalf("LineForVersion(%q) did not find a header", tt.newVersion)
			}
			if _, ok := p.Entry(tt.newVersion); !ok {
				t.Fatalf("Entry(%q) did not find a header", tt.newVersion)
			}
			result, ok := p.Between(tt.oldVersion, tt.newVersion)
			if !ok {
				t.Fatal("expected result")
			}
			if result != tt.want {
				t.Errorf("Between() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	p, err := ParseFile(filepath.Join("testdata", "keep_a_changelog.md"))
	if err != nil {
		t.Fatal(err)
	}
	versions := p.Versions()
	if len(versions) != 4 {
		t.Fatalf("expected 4 versions, got %d", len(versions))
	}
	if versions[3] != "1.0.0" {
		t.Errorf("expected last version 1.0.0, got %s", versions[3])
	}
}

func TestFindChangelog(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		path, err := FindChangelog(dir)
		if err != nil {
			t.Fatal(err)
		}
		if path != "" {
			t.Errorf("expected empty path, got %q", path)
		}
	})

	t.Run("finds changelog.md", func(t *testing.T) {
		dir := t.TempDir()
		content := "## [1.0.0] - 2024-01-01\n\nSome content that is long enough to pass the size check, we need at least one hundred bytes here to make sure.\n"
		if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		path, err := FindChangelog(dir)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(path) != "CHANGELOG.md" {
			t.Errorf("expected CHANGELOG.md, got %s", path)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("prerelease version", func(t *testing.T) {
		p := Parse("## [1.0.0-beta.1] - 2024-01-01\n\nBeta content")
		_, ok := p.Entry("1.0.0-beta.1")
		if !ok {
			t.Error("1.0.0-beta.1 not found")
		}
	})

	t.Run("build metadata", func(t *testing.T) {
		p := Parse("## [1.0.0+build.123] - 2024-01-01\n\nBuild content")
		_, ok := p.Entry("1.0.0+build.123")
		if !ok {
			t.Error("1.0.0+build.123 not found")
		}
	})

	t.Run("complex prerelease", func(t *testing.T) {
		p := Parse("## [2.0.0-x.7.z.92] - 2024-01-01\n\nComplex prerelease")
		_, ok := p.Entry("2.0.0-x.7.z.92")
		if !ok {
			t.Error("2.0.0-x.7.z.92 not found")
		}
	})

	t.Run("empty version content", func(t *testing.T) {
		p := Parse("## [2.0.0] - 2024-02-01\n\n## [1.0.0] - 2024-01-01\n\nSome content\n")
		entry, ok := p.Entry("2.0.0")
		if !ok {
			t.Fatal("2.0.0 not found")
		}
		if entry.Content != "" {
			t.Errorf("expected empty content for 2.0.0, got %q", entry.Content)
		}
		entry, _ = p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "Some content") {
			t.Error("expected 1.0.0 to contain 'Some content'")
		}
	})

	t.Run("preserves version order", func(t *testing.T) {
		p := Parse("## [3.0.0] - 2024-03-01\n## [1.0.0] - 2024-01-01\n## [2.0.0] - 2024-02-01\n")
		versions := p.Versions()
		want := []string{"3.0.0", "1.0.0", "2.0.0"}
		if len(versions) != len(want) {
			t.Fatalf("expected %d versions, got %d", len(want), len(versions))
		}
		for i, v := range want {
			if versions[i] != v {
				t.Errorf("version[%d] = %q, want %q", i, versions[i], v)
			}
		}
	})

}

func TestEdgeCasesContent(t *testing.T) {
	t.Run("preserves markdown links", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01\n\n- Added [feature](https://example.com)\n- See [docs](https://docs.example.com) for details\n")
		entry, _ := p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "[feature](https://example.com)") {
			t.Error("expected content to contain markdown link")
		}
	})

	t.Run("preserves inline code", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01\n\n- Fixed `bug_in_function` method\n")
		entry, _ := p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "`bug_in_function`") {
			t.Error("expected content to contain inline code")
		}
	})

	t.Run("ignores link references", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01\n\nContent here\n\n[1.0.0]: https://github.com/example/repo/releases/tag/v1.0.0\n")
		versions := p.Versions()
		if len(versions) != 1 {
			t.Errorf("expected 1 version, got %d: %v", len(versions), versions)
		}
		entry, _ := p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "[1.0.0]: https://github.com") {
			t.Error("expected content to contain link reference")
		}
	})

	t.Run("mixed list markers", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01\n\n- Dash item\n* Asterisk item\n- Another dash\n")
		entry, _ := p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "- Dash item") {
			t.Error("expected content to contain dash item")
		}
		if !strings.Contains(entry.Content, "* Asterisk item") {
			t.Error("expected content to contain asterisk item")
		}
	})

	t.Run("nested lists", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01\n\n- Main item\n  - Sub item one\n  - Sub item two\n")
		entry, _ := p.Entry("1.0.0")
		if !strings.Contains(entry.Content, "- Sub item one") {
			t.Error("expected content to contain sub item")
		}
	})

	t.Run("v prefix stripped", func(t *testing.T) {
		p := ParseWithFormat("## v1.0.0\n\nContent", FormatMarkdown)
		_, ok := p.Entry("1.0.0")
		if !ok {
			t.Error("1.0.0 not found (v prefix should be stripped)")
		}
	})

	t.Run("unreleased section", func(t *testing.T) {
		p := Parse("## [Unreleased]\n\n- Work in progress\n\n## [1.0.0] - 2024-01-01\n\n- Released feature\n")
		entry, ok := p.Entry("Unreleased")
		if !ok {
			t.Fatal("Unreleased not found")
		}
		if entry.Date != nil {
			t.Error("expected nil date for Unreleased")
		}
		if !strings.Contains(entry.Content, "Work in progress") {
			t.Error("expected Unreleased content to contain 'Work in progress'")
		}
	})

	t.Run("version with label", func(t *testing.T) {
		p := Parse("## [1.0.0] - 2024-01-01 - Codename Phoenix\n\nContent")
		entry, ok := p.Entry("1.0.0")
		if !ok {
			t.Fatal("1.0.0 not found")
		}
		assertDate(t, entry.Date, time.January, 1)
	})
}

func TestComprehensiveFixture(t *testing.T) {
	content := mustReadFixture(t, "comprehensive.md")
	p := Parse(content)

	versions := p.Versions()
	if len(versions) != 8 {
		t.Fatalf("expected 8 versions, got %d: %v", len(versions), versions)
	}

	wantVersions := []string{
		"Unreleased", "2.0.0-x.7.z.92", "1.5.0-beta.2", "1.4.0-rc.1",
		"1.3.0", "1.2.0", "1.1.0", "1.0.0",
	}
	for _, v := range wantVersions {
		if _, ok := p.Entry(v); !ok {
			t.Errorf("version %q not found", v)
		}
	}
}

func assertDate(t *testing.T, got *time.Time, month time.Month, day int) {
	t.Helper()
	if got == nil {
		t.Fatal("expected non-nil date")
	}
	const year = 2024
	if got.Year() != year || got.Month() != month || got.Day() != day {
		t.Errorf("date = %v, want %d-%02d-%02d", got, year, month, day)
	}
}
