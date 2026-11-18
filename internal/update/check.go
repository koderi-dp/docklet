package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"docklet/internal/model"
)

// LatestReleaseURL is the GitHub API endpoint for the newest published (non-draft, non-pre-release) release.
const LatestReleaseURL = "https://api.github.com/repos/koderi-dp/docklet/releases/latest"

const (
	requestTimeout = 10 * time.Second
	maxNotesRunes  = 1500
	maxBodyBytes   = 1 << 20
)

// Checker asks GitHub for the latest release. The request carries no credentials and no user data.
type Checker struct {
	URL    string       // defaults to LatestReleaseURL
	Client *http.Client // defaults to a client with a 10s timeout
	Now    func() time.Time
}

type release struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Check compares the running version with the latest release. It never returns an error:
// failures are described in the status so the UI can show them calmly.
func (c *Checker) Check(ctx context.Context, current string) model.UpdateStatus {
	st := model.UpdateStatus{CurrentVersion: current, CheckedAt: c.now().UnixMilli()}
	url := c.URL
	if url == "" {
		url = LatestReleaseURL
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		st.Error = "Could not build the update request."
		return st
	}
	req.Header.Set("User-Agent", "docklet")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		st.Error = "Could not reach GitHub. Check your internet connection and try again."
		return st
	}
	defer resp.Body.Close()
	return c.fromResponse(st, resp)
}

func (c *Checker) fromResponse(st model.UpdateStatus, resp *http.Response) model.UpdateStatus {
	switch {
	case resp.StatusCode == http.StatusOK:
		var r release
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&r); err != nil || r.TagName == "" {
			st.Error = "GitHub returned a response Docklet could not read."
			return st
		}
		st.LatestVersion = strings.TrimPrefix(r.TagName, "v")
		st.UpdateAvailable = IsNewer(st.LatestVersion, st.CurrentVersion)
		st.ReleaseURL = safeReleaseURL(r.HTMLURL)
		st.ReleaseNotes = plainNotes(r.Body)
		st.PublishedAt = r.PublishedAt
	case resp.StatusCode == http.StatusNotFound:
		st.Message = "No published releases found."
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden && rateLimited(resp):
		st.Error = rateLimitMessage(resp, c.now())
	default:
		st.Error = fmt.Sprintf("GitHub answered with an unexpected status (%d).", resp.StatusCode)
	}
	return st
}

func rateLimited(resp *http.Response) bool {
	return resp.Header.Get("Retry-After") != "" || resp.Header.Get("X-RateLimit-Remaining") == "0"
}

func rateLimitMessage(resp *http.Response, now time.Time) string {
	wait := time.Duration(0)
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		wait = time.Duration(s) * time.Second
	} else if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		wait = time.Unix(reset, 0).Sub(now)
	}
	if wait <= 0 {
		return "GitHub is rate limiting requests. Try again later."
	}
	return fmt.Sprintf("GitHub is rate limiting requests. Try again in %d min.", int((wait+time.Minute-1)/time.Minute))
}

// safeReleaseURL only passes through github.com links: the URL is opened in the browser.
func safeReleaseURL(u string) string {
	if strings.HasPrefix(u, "https://github.com/") {
		return u
	}
	return ""
}

var (
	mdLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdHeading = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	mdMarks   = strings.NewReplacer("**", "", "__", "", "`", "")
)

// plainNotes turns release-note markdown into short plain text.
func plainNotes(body string) string {
	s := strings.ReplaceAll(body, "\r\n", "\n")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdHeading.ReplaceAllString(s, "")
	s = strings.TrimSpace(mdMarks.Replace(s))
	if r := []rune(s); len(r) > maxNotesRunes {
		s = strings.TrimSpace(string(r[:maxNotesRunes])) + "…"
	}
	return s
}
