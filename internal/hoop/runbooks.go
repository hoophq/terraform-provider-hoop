package hoop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type RunbookConfig struct {
	ID           string        `json:"id"`
	Repositories []RunbookRepo `json:"repositories"`
}

type RunbookRepo struct {
	Repository    string `json:"repository"`
	GitURL        string `json:"git_url"`
	GitUser       string `json:"git_user"`
	GitPassword   string `json:"git_password"`
	GitHookTTL    int32  `json:"git_hook_ttl"`
	SSHUser       string `json:"ssh_user"`
	SSHKey        string `json:"ssh_key"`
	SSHKeyPass    string `json:"ssh_keypass"`
	SSHKnownHosts string `json:"ssh_known_hosts"`
}

func (c *Client) GetRunbookConfigByURL(gitURL string) (*RunbookRepo, error) {
	apiURL := fmt.Sprintf("%s/runbooks/configurations", c.apiURL)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var resource RunbookConfig
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding runbooks configuration resource, reason=%v", err)
		}
		for _, repo := range resource.Repositories {
			if repo.GitURL == gitURL {
				return &repo, nil
			}
		}
		return nil, fmt.Errorf("git repository %q not found", gitURL)
	}
	return nil, validateErr(resp)
}

func (c *Client) CreateRunbookRepo(repo RunbookRepo) (*RunbookRepo, error) {
	return c.doRunbookRequestWithBody("", repo)
}

func (c *Client) UpdateRunbookRepoByID(repo RunbookRepo) (*RunbookRepo, error) {
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(repo.GitURL)).String()
	return c.doRunbookRequestWithBody(id, repo)
}

func (c *Client) DeleteRunbookRepoByID(gitURL string) error {
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(gitURL)).String()
	apiURL := fmt.Sprintf("%s/runbooks/configurations/%s", c.apiURL, id)
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return validateErr(resp)
}

// doRunbookRequestWithBody issue a POST or PUT request to the runbook API with the provided body.
// If id is empty, it will issue a POST request to create a new runbook configuration.
// If id is provided, it will issue a PUT request to update the existing runbook configuration.
func (c *Client) doRunbookRequestWithBody(id string, repo RunbookRepo) (*RunbookRepo, error) {
	method := "POST"
	apiURL := fmt.Sprintf("%s/runbooks/configurations", c.apiURL)
	if id != "" {
		method = "PUT"
		apiURL = fmt.Sprintf("%s/%s", apiURL, id)
	}
	jsonData, err := json.Marshal(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runbook repository configuration, reason=%v", err)
	}
	req, err := http.NewRequest(method, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var resource RunbookRepo
		if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
			return nil, fmt.Errorf("failed decoding runbook repository resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}
