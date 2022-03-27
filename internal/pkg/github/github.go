// Copyright 2020-2022 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-github/v42/github"
	"golang.org/x/oauth2"
)

// CompareCommitsStatus is the result of comparing two commits.
type CompareCommitsStatus int

// The possible values for returned from Client.CompareCommits.
// see https://stackoverflow.com/a/23969867
const (
	CompareCommitsStatusDiverged CompareCommitsStatus = iota + 1
	CompareCommitsStatusIdentical
	CompareCommitsStatusAhead
	CompareCommitsStatusBehind
)

var (
	compareCommitStatusStrings = map[CompareCommitsStatus]string{
		CompareCommitsStatusDiverged:  "diverged",
		CompareCommitsStatusIdentical: "identical",
		CompareCommitsStatusAhead:     "ahead",
		CompareCommitsStatusBehind:    "behind",
	}

	stringsToCompareCommitStatus = map[string]CompareCommitsStatus{
		"diverged":  CompareCommitsStatusDiverged,
		"identical": CompareCommitsStatusIdentical,
		"ahead":     CompareCommitsStatusAhead,
		"behind":    CompareCommitsStatusBehind,
	}
)

func (s CompareCommitsStatus) String() string {
	got, ok := compareCommitStatusStrings[s]
	if !ok {
		return fmt.Sprintf("unknown(%d)", s)
	}
	return got
}

type Client struct {
	client *github.Client
	owner  string
	repo   string
}

// NewClient returns a new github client.
// baseURL is optional and defaults to https://api.github.com/.
func NewClient(ctx context.Context, token, userAgent, owner, repository string, baseURL *url.URL) *Client {
	goGithubClient := github.NewClient(
		oauth2.NewClient(
			ctx,
			oauth2.StaticTokenSource(
				&oauth2.Token{
					AccessToken: token,
				},
			),
		),
	)

	if baseURL != nil {
		*goGithubClient.BaseURL = *baseURL
		// The underlying go-github client library requires the url has a trailing slash, but the
		// value of GITHUB_API_URL usually doesn't have one.
		if !strings.HasSuffix(goGithubClient.BaseURL.Path, "/") {
			goGithubClient.BaseURL.Path += "/"
		}
	}

	goGithubClient.UserAgent = userAgent
	return &Client{
		client: goGithubClient,
		owner:  owner,
		repo:   repository,
	}
}

// CompareCommits compares a range of commits with each other.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/commits#compare-two-commits
func (c *Client) CompareCommits(ctx context.Context, base string, head string) (CompareCommitsStatus, error) {
	comp, _, err := c.client.Repositories.CompareCommits(ctx, c.owner, c.repo, base, head, nil)
	if err != nil {
		return 0, err
	}
	status, ok := stringsToCompareCommitStatus[comp.GetStatus()]
	if !ok {
		return 0, fmt.Errorf("unknown CompareCommitsStatus: %s", comp.GetStatus())
	}
	return status, nil
}

// IsResponseError returns true if the error is a *github.ErrorResponse with the given status code.
func IsResponseError(statusCode int, err error) bool {
	var errorResponse *github.ErrorResponse
	if !errors.As(err, &errorResponse) {
		return false
	}
	if errorResponse.Response == nil {
		return false
	}
	return errorResponse.Response.StatusCode == statusCode
}
