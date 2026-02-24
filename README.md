# changelog

A Go library for parsing changelog files into structured entries. Supports Keep a Changelog, markdown header, and setext/underline formats with automatic detection.

Port of the Ruby [changelog-parser](https://github.com/git-pkgs/changelog-parser) gem.

## Installation

```bash
go get github.com/git-pkgs/changelog
```

## Usage

### Parse a string

```go
p := changelog.Parse(content)

for _, v := range p.Versions() {
    entry, _ := p.Entry(v)
    fmt.Printf("%s (%v): %s\n", v, entry.Date, entry.Content)
}
```

### Parse a file

```go
p, err := changelog.ParseFile("CHANGELOG.md")
```

### Find and parse a changelog in a directory

```go
p, err := changelog.FindAndParse(".")
```

Searches for common changelog filenames (CHANGELOG.md, NEWS, CHANGES, HISTORY, etc.) and parses the first match.

### Specify format explicitly

```go
p := changelog.ParseWithFormat(content, changelog.FormatKeepAChangelog)
p := changelog.ParseWithFormat(content, changelog.FormatMarkdown)
p := changelog.ParseWithFormat(content, changelog.FormatUnderline)
```

### Custom regex pattern

```go
pattern := regexp.MustCompile(`^Version ([\d.]+) released (\d{4}-\d{2}-\d{2})`)
p := changelog.ParseWithPattern(content, pattern)
```

The first capture group is the version string. An optional second capture group is parsed as a date (YYYY-MM-DD).

### Get content between versions

```go
content, ok := p.Between("1.0.0", "2.0.0")
```

### Fetch and parse from a repository URL

```go
p, err := changelog.FetchAndParse(ctx, "https://github.com/owner/repo", "CHANGELOG.md")
```

Constructs a raw content URL (GitHub and GitLab are supported), fetches the file, and parses it.

You can also build the raw URL yourself:

```go
url, err := changelog.RawContentURL("https://github.com/owner/repo", "CHANGELOG.md")
// "https://raw.githubusercontent.com/owner/repo/HEAD/CHANGELOG.md"
```

### Find line number for a version

```go
line := p.LineForVersion("1.0.0") // 0-based, -1 if not found
```

## Supported formats

**Keep a Changelog** (`## [1.0.0] - 2024-01-15`):

```markdown
## [Unreleased]

## [1.1.0] - 2024-03-15
### Added
- New feature

## [1.0.0] - 2024-01-15
- Initial release
```

**Markdown headers** (`## 1.0.0 (2024-01-15)` or `### v1.0.0`):

```markdown
## 2.0.0 (2024-03-01)
- Breaking changes

## 1.5.0
- New features
```

**Setext/underline** (version with `===` or `---` underline):

```markdown
3.0.0
=====
Major release.

2.1.0
-----
Minor release.
```

## License

MIT
