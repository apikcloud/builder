// SPDX-License-Identifier: MIT
package enterprise

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiOwner and apiRepo are RepoURL's GitHub coordinates, used for the REST
// API calls ResolveCommit and Download make (date-based commit lookup and
// zipball download) as an alternative to a full git clone.
const (
	apiOwner = "odoo"
	apiRepo  = "enterprise"
)

// APIBaseURL is the GitHub API root ResolveCommit and Download call
// against. Overridable so tests can point it at a local fixture server;
// production code never reassigns it.
var APIBaseURL = "https://api.github.com"

// ResolveCommit returns the SHA of the newest commit on branch at or
// before date (YYYYMMDD) in the Enterprise repository, so a build can pin
// Enterprise addons to the same day as the community base image
// (base.release) instead of a branch's ever-moving tip.
func ResolveCommit(branch, date, token string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("enterprise: base.version must be set to resolve an Enterprise commit by date")
	}
	if len(date) != 8 {
		return "", fmt.Errorf("enterprise: date %q must be in YYYYMMDD format", date)
	}
	if token == "" {
		return "", fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}
	until := fmt.Sprintf("%s-%s-%sT23:59:59Z", date[:4], date[4:6], date[6:8])

	url := fmt.Sprintf("%s/repos/%s/%s/commits?sha=%s&until=%s", APIBaseURL, apiOwner, apiRepo, branch, until)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("enterprise: building commit lookup request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("enterprise: resolving commit on %s before %s: %w", branch, date, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("enterprise: reading commit lookup response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("enterprise: commit lookup on %s before %s failed: %s", branch, date, resp.Status)
	}

	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &commits); err != nil {
		return "", fmt.Errorf("enterprise: parsing commit lookup response: %w", err)
	}
	if len(commits) == 0 {
		return "", fmt.Errorf("enterprise: no commit found on %s at or before %s", branch, date)
	}

	return commits[0].SHA, nil
}
