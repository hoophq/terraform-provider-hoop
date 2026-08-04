// Copyright (c) HashiCorp, Inc.

package hoop

import (
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
	req, err := c.newRequest("GET", "/runbooks/configurations", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var resource RunbookConfig
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding runbooks configuration resource, reason=%v", err)
	}
	for _, repo := range resource.Repositories {
		if repo.GitURL == gitURL {
			return &repo, nil
		}
	}
	return nil, fmt.Errorf("git repository %q not found", gitURL)
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
	req, err := c.newRequest("DELETE", "/runbooks/configurations/"+id, nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

// doRunbookRequestWithBody issue a POST or PUT request to the runbook API with the provided body.
// If id is empty, it will issue a POST request to create a new runbook configuration.
// If id is provided, it will issue a PUT request to update the existing runbook configuration.
func (c *Client) doRunbookRequestWithBody(id string, repo RunbookRepo) (*RunbookRepo, error) {
	method := "POST"
	path := "/runbooks/configurations"
	if id != "" {
		method = "PUT"
		path += "/" + id
	}
	jsonData, err := json.Marshal(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runbook repository configuration, reason=%v", err)
	}
	req, err := c.newRequest(method, path, jsonData)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var resource RunbookRepo
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding runbook repository resource, reason=%v", err)
	}
	return &resource, nil
}
