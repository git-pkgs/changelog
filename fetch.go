package changelog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RawContentURL constructs a URL that serves the raw content of a file in a
// repository. Supports GitHub and GitLab. The repoURL should be the repository's
// web URL (e.g. "https://github.com/owner/repo"). Trailing ".git" suffixes and
// slashes are stripped automatically.
func RawContentURL(repoURL, filename string) (string, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimSuffix(repoURL, "/")

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parsing repository URL: %w", err)
	}

	parts := strings.SplitN(strings.TrimPrefix(parsed.Path, "/"), "/", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("cannot parse owner/repo from %s", repoURL)
	}
	owner := parts[0]
	repo := parts[1]

	switch parsed.Host {
	case "github.com":
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s", owner, repo, filename), nil
	case "gitlab.com":
		return fmt.Sprintf("https://gitlab.com/%s/%s/-/raw/HEAD/%s", owner, repo, filename), nil
	default:
		return "", fmt.Errorf("unsupported host %s (only github.com and gitlab.com are supported)", parsed.Host)
	}
}

// FetchAndParse fetches a changelog from a repository and parses it.
// It constructs the raw content URL from the repository URL and changelog
// filename, fetches the content over HTTP, and returns a Parser.
func FetchAndParse(ctx context.Context, repoURL, filename string) (*Parser, error) {
	rawURL, err := RawContentURL(repoURL, filename)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return Parse(string(body)), nil
}
