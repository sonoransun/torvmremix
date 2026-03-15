package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/user/extorvm/controller/internal/logging"
)

// Version is the current application version. Set at build time via ldflags.
var Version = "0.1.0"

// GitHubRelease represents a subset of the GitHub Releases API response.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// UpdateInfo describes an available update.
type UpdateInfo struct {
	Available   bool
	NewVersion  string
	ReleaseURL  string
	DownloadURL string // platform-specific binary
}

// UpdateObserver is called when update status changes.
type UpdateObserver func(UpdateInfo)

// Checker periodically checks for new releases via the GitHub API.
type Checker struct {
	repoOwner string
	repoName  string
	logger    *logging.Logger
	interval  time.Duration

	mu        sync.Mutex
	observers []UpdateObserver
	latest    UpdateInfo
	done      chan struct{}
}

// NewChecker creates an update checker for the given GitHub repository.
func NewChecker(repoOwner, repoName string, logger *logging.Logger) *Checker {
	return &Checker{
		repoOwner: repoOwner,
		repoName:  repoName,
		logger:    logger,
		interval:  24 * time.Hour,
		done:      make(chan struct{}),
	}
}

// OnUpdate registers a callback for update availability changes.
func (c *Checker) OnUpdate(fn UpdateObserver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observers = append(c.observers, fn)
}

// Latest returns the most recently discovered update info.
func (c *Checker) Latest() UpdateInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest
}

// Start begins periodic update checks in the background.
// Checks immediately, then every interval.
func (c *Checker) Start() {
	go c.loop()
}

// Stop stops the periodic checker.
func (c *Checker) Stop() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func (c *Checker) loop() {
	c.check()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.check()
		case <-c.done:
			return
		}
	}
}

func (c *Checker) check() {
	info, err := c.fetchLatest()
	if err != nil {
		c.logger.Debug("update check: %v", err)
		return
	}

	c.mu.Lock()
	changed := info.Available != c.latest.Available || info.NewVersion != c.latest.NewVersion
	c.latest = info
	var snap []UpdateObserver
	if changed {
		snap = make([]UpdateObserver, len(c.observers))
		copy(snap, c.observers)
	}
	c.mu.Unlock()

	if changed && info.Available {
		c.logger.Info("update available: %s → %s (%s)", Version, info.NewVersion, info.ReleaseURL)
	}

	for _, fn := range snap {
		fn(info)
	}
}

func (c *Checker) fetchLatest() (UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", c.repoOwner, c.repoName)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("github API status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{}, fmt.Errorf("decode release: %w", err)
	}

	tag := strings.TrimPrefix(release.TagName, "v")
	if tag == "" {
		return UpdateInfo{}, fmt.Errorf("empty release tag")
	}

	info := UpdateInfo{
		Available:  compareVersions(Version, tag) < 0,
		NewVersion: tag,
		ReleaseURL: release.HTMLURL,
	}

	return info, nil
}

// compareVersions compares two semantic version strings.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var av, bv int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &av)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bv)
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}
