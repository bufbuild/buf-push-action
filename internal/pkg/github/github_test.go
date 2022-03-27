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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const (
	testGithubToken      = "githubToken"
	testGithubOwner      = "owner"
	testGithubRepository = "repo"
	testUserAgent        = "userAgent"
)

func TestCompareCommits(t *testing.T) {
	assertRequestHeaders := func(t *testing.T, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("Bearer %s", testGithubToken), r.Header.Get("Authorization"))
		assert.Equal(t, testUserAgent, r.Header.Get("User-Agent"))
		assert.Equal(t, "GET", r.Method)
	}
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		server := newTestServer(t)
		server.addHandler("/repos/owner/repo/compare/foo...bar", func(w http.ResponseWriter, r *http.Request) {
			assertRequestHeaders(t, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ahead_by":  1,
				"behind_by": 2,
				"status":    CompareCommitsStatusDiverged.String(),
			})
			assert.NoError(t, err)
		})
		client := server.client(ctx)
		status, err := client.CompareCommits(ctx, "foo", "bar")
		require.NoError(t, err)
		assert.Equal(t, CompareCommitsStatusDiverged, status)
	})

	t.Run("404", func(t *testing.T) {
		ctx := context.Background()
		server := newTestServer(t)
		server.handlers["/repos/owner/repo/compare/foo...bar"] = func(w http.ResponseWriter, r *http.Request) {
			assertRequestHeaders(t, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"message":           "Not Found",
				"documentation_url": "https://developer.github.com/v3/repos/commits/#compare-two-commits",
			})
			assert.NoError(t, err)
		}
		client := server.client(ctx)
		_, err := client.CompareCommits(ctx, "foo", "bar")
		require.Error(t, err)
		errorResponse, ok := err.(*github.ErrorResponse)
		require.True(t, ok)
		require.Equal(t, http.StatusNotFound, errorResponse.Response.StatusCode)
	})

	t.Run("unknown status", func(t *testing.T) {
		ctx := context.Background()
		server := newTestServer(t)
		server.addHandler("/repos/owner/repo/compare/foo...bar", func(w http.ResponseWriter, r *http.Request) {
			assertRequestHeaders(t, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ahead_by":  1,
				"behind_by": 2,
				"status":    "something unexpected",
			})
			assert.NoError(t, err)
		})
		client := server.client(ctx)
		_, err := client.CompareCommits(ctx, "foo", "bar")
		require.EqualError(t, err, "unknown CompareCommitsStatus: something unexpected")
	})
}

func TestNewClient(t *testing.T) {
	baseURLString := "https://api.github.com"
	baseURL, err := url.Parse(baseURLString)
	require.NoError(t, err)
	ctx := context.Background()
	client := NewClient(ctx, testGithubToken, testUserAgent, testGithubOwner, testGithubRepository, baseURL)
	// make sure the baseURL has a trailing slash
	assert.Equal(t, "https://api.github.com/", client.client.BaseURL.String())
	// make sure the original baseURL is not modified
	assert.Equal(t, baseURLString, baseURL.String())
}

func TestIsResponseError(t *testing.T) {
	assert.False(t, IsResponseError(http.StatusNotFound, nil))
	assert.False(t, IsResponseError(http.StatusNotFound, assert.AnError))
	assert.False(t, IsResponseError(http.StatusNotFound, &github.ErrorResponse{}))
	assert.False(t, IsResponseError(http.StatusNotFound, &github.ErrorResponse{
		Response: &http.Response{
			StatusCode: http.StatusBadRequest,
		},
	}))
	assert.True(t, IsResponseError(http.StatusNotFound, &github.ErrorResponse{
		Response: &http.Response{
			StatusCode: http.StatusNotFound,
		},
	}))
}

type testServer struct {
	t        *testing.T
	handlers map[string]http.HandlerFunc
	server   *httptest.Server
}

func newTestServer(t *testing.T) *testServer {
	ts := &testServer{
		t:        t,
		handlers: map[string]http.HandlerFunc{},
	}
	ts.server = httptest.NewTLSServer(ts)
	t.Cleanup(ts.server.Close)
	return ts
}

func (t *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, ok := t.handlers[r.URL.Path]
	if !ok {
		t.t.Errorf("unexpected request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	handler(w, r)
}

func (t *testServer) client(ctx context.Context) *Client {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, t.server.Client())
	baseURL, err := url.Parse(t.server.URL)
	assert.NoError(t.t, err)
	return NewClient(ctx, testGithubToken, testUserAgent, testGithubOwner, testGithubRepository, baseURL)
}

func (t *testServer) addHandler(path string, handler http.HandlerFunc) {
	if t.handlers == nil {
		t.handlers = make(map[string]http.HandlerFunc)
	}
	t.handlers[path] = handler
}
