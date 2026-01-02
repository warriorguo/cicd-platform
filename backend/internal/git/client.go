package git

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"encoding/json"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

type ProviderType string

const (
	GitHub ProviderType = "github"
	GitLab ProviderType = "gitlab"
	Gitea  ProviderType = "gitea"
)

type RepoInfo struct {
	Provider ProviderType
	Owner    string
	Repo     string
	BaseURL  string
}

func ParseGitURL(gitURL string) (*RepoInfo, error) {
	gitURL = strings.TrimSuffix(gitURL, ".git")
	
	// SSH format: git@github.com:owner/repo
	sshRegex := regexp.MustCompile(`^git@([^:]+):(.+)/(.+)$`)
	if matches := sshRegex.FindStringSubmatch(gitURL); len(matches) == 4 {
		host := matches[1]
		owner := matches[2]
		repo := matches[3]
		
		provider := detectProvider(host)
		baseURL := fmt.Sprintf("https://%s", host)
		
		return &RepoInfo{
			Provider: provider,
			Owner:    owner,
			Repo:     repo,
			BaseURL:  baseURL,
		}, nil
	}

	// HTTPS format
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		return nil, fmt.Errorf("invalid git URL: %w", err)
	}

	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid repository path: %s", parsedURL.Path)
	}

	provider := detectProvider(parsedURL.Host)
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	return &RepoInfo{
		Provider: provider,
		Owner:    pathParts[0],
		Repo:     pathParts[1],
		BaseURL:  baseURL,
	}, nil
}

func detectProvider(host string) ProviderType {
	switch {
	case strings.Contains(host, "github"):
		return GitHub
	case strings.Contains(host, "gitlab"):
		return GitLab
	default:
		return Gitea
	}
}

func (c *Client) ListBranches(ctx context.Context, repoInfo *RepoInfo, token string) ([]string, error) {
	switch repoInfo.Provider {
	case GitHub:
		return c.listGitHubBranches(ctx, repoInfo, token)
	case GitLab:
		return c.listGitLabBranches(ctx, repoInfo, token)
	default:
		return c.listGiteaBranches(ctx, repoInfo, token)
	}
}

func (c *Client) CheckDockerfile(ctx context.Context, repoInfo *RepoInfo, branch, dockerfilePath, token string) (bool, error) {
	switch repoInfo.Provider {
	case GitHub:
		return c.checkGitHubFile(ctx, repoInfo, branch, dockerfilePath, token)
	case GitLab:
		return c.checkGitLabFile(ctx, repoInfo, branch, dockerfilePath, token)
	default:
		return c.checkGiteaFile(ctx, repoInfo, branch, dockerfilePath, token)
	}
}

func (c *Client) listGitHubBranches(ctx context.Context, repoInfo *RepoInfo, token string) ([]string, error) {
	url := fmt.Sprintf("%s/api/repos/%s/%s/branches", repoInfo.BaseURL, repoInfo.Owner, repoInfo.Repo)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}

	result := make([]string, len(branches))
	for i, branch := range branches {
		result[i] = branch.Name
	}
	return result, nil
}

func (c *Client) checkGitHubFile(ctx context.Context, repoInfo *RepoInfo, branch, dockerfilePath, token string) (bool, error) {
	url := fmt.Sprintf("%s/api/repos/%s/%s/contents/%s?ref=%s", 
		repoInfo.BaseURL, repoInfo.Owner, repoInfo.Repo, dockerfilePath, branch)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) listGitLabBranches(ctx context.Context, repoInfo *RepoInfo, token string) ([]string, error) {
	projectPath := url.QueryEscape(fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo))
	url := fmt.Sprintf("%s/api/v4/projects/%s/repository/branches", repoInfo.BaseURL, projectPath)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API error: %d", resp.StatusCode)
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}

	result := make([]string, len(branches))
	for i, branch := range branches {
		result[i] = branch.Name
	}
	return result, nil
}

func (c *Client) checkGitLabFile(ctx context.Context, repoInfo *RepoInfo, branch, dockerfilePath, token string) (bool, error) {
	projectPath := url.QueryEscape(fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo))
	filePath := url.QueryEscape(dockerfilePath)
	url := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s?ref=%s", 
		repoInfo.BaseURL, projectPath, filePath, branch)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) listGiteaBranches(ctx context.Context, repoInfo *RepoInfo, token string) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/branches", repoInfo.BaseURL, repoInfo.Owner, repoInfo.Repo)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gitea API error: %d", resp.StatusCode)
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}

	result := make([]string, len(branches))
	for i, branch := range branches {
		result[i] = branch.Name
	}
	return result, nil
}

func (c *Client) checkGiteaFile(ctx context.Context, repoInfo *RepoInfo, branch, dockerfilePath, token string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s?ref=%s", 
		repoInfo.BaseURL, repoInfo.Owner, repoInfo.Repo, dockerfilePath, branch)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}