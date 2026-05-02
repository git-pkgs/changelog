package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRawContentURL(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		filename string
		want     string
		wantErr  bool
	}{
		{
			name:     "github https",
			repoURL:  "https://github.com/olivierlacan/keep-a-changelog",
			filename: "CHANGELOG.md",
			want:     "https://raw.githubusercontent.com/olivierlacan/keep-a-changelog/HEAD/CHANGELOG.md",
		},
		{
			name:     "github with trailing .git",
			repoURL:  "https://github.com/lodash/lodash.git",
			filename: "CHANGELOG.md",
			want:     "https://raw.githubusercontent.com/lodash/lodash/HEAD/CHANGELOG.md",
		},
		{
			name:     "github with trailing slash",
			repoURL:  "https://github.com/lodash/lodash/",
			filename: "CHANGELOG.md",
			want:     "https://raw.githubusercontent.com/lodash/lodash/HEAD/CHANGELOG.md",
		},
		{
			name:     "gitlab https",
			repoURL:  "https://gitlab.com/inkscape/inkscape",
			filename: "NEWS.md",
			want:     "https://gitlab.com/inkscape/inkscape/-/raw/HEAD/NEWS.md",
		},
		{
			name:     "gitlab with trailing .git",
			repoURL:  "https://gitlab.com/inkscape/inkscape.git",
			filename: "NEWS.md",
			want:     "https://gitlab.com/inkscape/inkscape/-/raw/HEAD/NEWS.md",
		},
		{
			name:     "unsupported host",
			repoURL:  "https://bitbucket.org/owner/repo",
			filename: "CHANGELOG.md",
			wantErr:  true,
		},
		{
			name:     "no path segments",
			repoURL:  "https://github.com/",
			filename: "CHANGELOG.md",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RawContentURL(tt.repoURL, tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFetchAndParse(t *testing.T) {
	changelogContent := "## [2.0.0] - 2024-03-01\n\nNew features\n\n## [1.0.0] - 2024-01-01\n\nInitial release\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(changelogContent))
	}))
	defer srv.Close()

	// We can't easily test with real GitHub/GitLab URLs, but we can test
	// the parsing side by testing FetchAndParse's error handling and
	// RawContentURL separately. For a real integration-like test, we'd
	// need to mock the URL construction. Instead, test that unsupported
	// hosts produce errors.
	t.Run("unsupported host returns error", func(t *testing.T) {
		_, err := FetchAndParse(context.Background(), "https://bitbucket.org/owner/repo", "CHANGELOG.md")
		if err == nil {
			t.Error("expected error for unsupported host")
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchAndParseLimitsBodySize(t *testing.T) {
	const limit = 1 << 20

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("## [1.0.0] - 2024-01-01\n\n"))
		_, _ = w.Write([]byte(strings.Repeat("a", limit*2)))
	}))
	defer srv.Close()

	srvURL, _ := url.Parse(srv.URL)

	origTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = srvURL.Scheme
		req.URL.Host = srvURL.Host
		return http.DefaultTransport.RoundTrip(req)
	})
	defer func() { http.DefaultClient.Transport = origTransport }()

	parser, err := FetchAndParse(context.Background(), "https://github.com/owner/repo", "CHANGELOG.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parser.content) > limit {
		t.Errorf("body not capped: got %d bytes, want <= %d", len(parser.content), limit)
	}
}
