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

package bufpushaction

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/rpc"
	gogithub "github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGitCommit1   = "fa1afe1cafefa1afe1cafefa1afe1cafefa1afe1"
	testGitCommit2   = "beefcafebeefcafebeefcafebeefcafebeefcafe"
	testBsrCommit    = "01234567890123456789012345678901"
	testModuleName   = "buf.build/foo/bar"
	testOwner        = "foo"
	testRepository   = "bar"
	testMainTrack    = "main"
	testAddress      = "buf.build"
	testNonMainTrack = "non-main"
)

type cmdTest struct {
	subCommand   string
	provider     fakeRegistryProvider
	config       string
	env          map[string]string
	errMsg       string
	stdout       []string
	outputs      map[string]string
	githubClient fakeGithubClient
}

func TestDeleteTrack(t *testing.T) {
	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_BUF_TOKEN": "",
		},
		errMsg: "a buf authentication token was not provided",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_INPUT": "path/does/not/exist",
		},
		errMsg: "path/does/not/exist: does not exist",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_INPUT": t.TempDir(),
		},
		errMsg: "module identity not found in config",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_INPUT": writeConfigFile(t, "invalid config"),
		},
		errMsg: "could not unmarshal as YAML",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_INPUT": writeConfigFile(t, v1Config("not-a-module")),
		},
		errMsg: `module identity "not-a-module" is invalid: must be in the form remote/owner/repository`,
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_TRACK": "",
		},
		errMsg: "track not provided",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_DEFAULT_BRANCH": "",
		},
		errMsg: "default_branch not provided",
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		env: map[string]string{
			"INPUT_TRACK": testMainTrack,
		},
		stdout: []string{
			"::notice::Skipping because the main track can not be deleted from BSR",
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		provider: fakeRegistryProvider{
			newRepositoryTrackServiceErr: assert.AnError,
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		provider: fakeRegistryProvider{
			deleteRepositoryTrackByNameErr: assert.AnError,
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		provider: fakeRegistryProvider{
			deleteRepositoryTrackByNameErr: rpc.NewNotFoundError("an error"),
		},
		errMsg: `"buf.build/foo/bar" does not exist`,
	})
}

func TestPush(t *testing.T) {
	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		outputs: map[string]string{
			"commit":     testBsrCommit,
			"commit_url": fmt.Sprintf("https://%s/tree/%s", testModuleName, testBsrCommit),
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "path/does/not/exist",
		},
		errMsg: "path/does/not/exist: does not exist",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		errMsg:     "module has no files",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
			"INPUT_TRACK": "",
		},
		errMsg: "track not provided",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT":          "./testdata/success",
			"INPUT_DEFAULT_BRANCH": "",
		},
		errMsg: "default_branch not provided",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT":          "./testdata/success",
			"INPUT_DEFAULT_BRANCH": testNonMainTrack,
			"INPUT_TRACK":          testMainTrack,
			githubRefNameKey:       testMainTrack,
		},
		errMsg: "cannot push to main track from a non-default branch",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		provider: fakeRegistryProvider{
			newRepositoryCommitServiceErr: assert.AnError,
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		provider: fakeRegistryProvider{
			getRepositoryCommitByReferenceErr: assert.AnError,
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		provider: fakeRegistryProvider{
			headTags: []string{"some", "other", "tags", strings.Repeat("z", 40)},
		},
		outputs: map[string]string{
			"commit":     testBsrCommit,
			"commit_url": fmt.Sprintf("https://%s/tree/%s", testModuleName, testBsrCommit),
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
			githubSHAKey:  "",
		},
		errMsg: "current git commit not found in environment",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		githubClient: fakeGithubClient{
			fakeCompareCommits: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit2,
					err:        githubNotFoundErr(),
				},
			},
		},
		outputs: map[string]string{
			"commit":     testBsrCommit,
			"commit_url": fmt.Sprintf("https://%s/tree/%s", testModuleName, testBsrCommit),
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		githubClient: fakeGithubClient{
			fakeCompareCommits: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit2,
					err:        assert.AnError,
				},
			},
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		githubClient: fakeGithubClient{
			fakeCompareCommits: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit2,
					status:     github.CompareCommitsStatusIdentical,
				},
			},
		},
		stdout: []string{
			"::notice::Skipping because the current git commit is already the head of track non-main",
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		githubClient: fakeGithubClient{
			fakeCompareCommits: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit2,
					status:     github.CompareCommitsStatusBehind,
				},
			},
		},
		stdout: []string{
			"::notice::Skipping because the current git commit is behind the head of track non-main",
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		githubClient: fakeGithubClient{
			fakeCompareCommits: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit2,
					status:     github.CompareCommitsStatusDiverged,
				},
			},
		},
		stdout: []string{
			"::notice::The current git commit is diverged from the head of track non-main",
		},
		outputs: map[string]string{
			"commit":     testBsrCommit,
			"commit_url": fmt.Sprintf("https://%s/tree/%s", testModuleName, testBsrCommit),
		},
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		githubClient: fakeGithubClient{
			fakeCompareCommits: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit2,
					status:     0,
				},
			},
		},
		errMsg: "unexpected status: unknown(0)",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		provider: fakeRegistryProvider{
			newPushServiceErr: assert.AnError,
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		provider: fakeRegistryProvider{
			pushErr: rpc.NewNotFoundError("omg"),
		},
		errMsg: "omg",
	})

	runCmdTest(t, cmdTest{
		subCommand: "push",
		env: map[string]string{
			"INPUT_INPUT": "./testdata/success",
		},
		provider: fakeRegistryProvider{
			pushErr: rpc.NewAlreadyExistsError("omg"),
		},
		stdout: []string{
			"::notice::The latest commit has the same content; not creating a new commit.",
		},
		outputs: map[string]string{
			"commit":     testBsrCommit,
			"commit_url": fmt.Sprintf("https://%s/tree/%s", testModuleName, testBsrCommit),
		},
	})
}

func runCmdTest(t *testing.T, test cmdTest) {
	var stdout, stderr bytes.Buffer
	test.provider.t = t
	test.githubClient.t = t
	if test.config == "" {
		test.config = v1Config(testModuleName)
	}
	env := test.env
	if env == nil {
		env = map[string]string{}
	}
	defaultEnv := map[string]string{
		"INPUT_BUF_TOKEN":      "buf-token",
		"INPUT_INPUT":          writeConfigFile(t, test.config),
		"INPUT_TRACK":          testNonMainTrack,
		"INPUT_DEFAULT_BRANCH": testMainTrack,
		"INPUT_GITHUB_TOKEN":   "github-token",
		githubRepositoryKey:    "github-owner/github-repo",
		githubRefNameKey:       testMainTrack,
		githubSHAKey:           testGitCommit2,
	}
	if test.provider.headTags == nil {
		test.provider.headTags = []string{testGitCommit1}
	}
	if test.provider.pushTags == nil {
		test.provider.pushTags = []string{testGitCommit2}
	}

	for k, v := range defaultEnv {
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}
	if len(test.githubClient.fakeCompareCommits) == 0 {
		test.githubClient.fakeCompareCommits = []fakeCompareCommits{
			{
				expectBase: testGitCommit1,
				expectHead: testGitCommit2,
				status:     github.CompareCommitsStatusAhead,
			},
		}
	}
	ctx := context.WithValue(context.Background(), registryProviderContextKey, &test.provider)
	ctx = context.WithValue(ctx, githubClientContextKey, &test.githubClient)
	container := app.NewContainer(env, nil, &stdout, &stderr, "test", test.subCommand)
	command := newRootCommand("test")
	err := appcmd.Run(ctx, container, command)
	if test.errMsg != "" {
		errMsg := fmt.Sprintf("::error::%s", test.errMsg)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), errMsg)
		}
		assert.Contains(t, stderr.String(), errMsg)
	} else {
		assert.NoError(t, err)
		assert.Empty(t, stderr.String())
	}

	output := map[string]string{}
	var stdoutLines []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "::set-output name=") {
			keyValue := strings.TrimPrefix(line, "::set-output name=")
			key, value, ok := strings.Cut(keyValue, "::")
			if !ok {
				stdoutLines = append(stdoutLines, line)
				continue
			}
			output[key] = value
			continue
		}
		stdoutLines = append(stdoutLines, line)
	}
	assert.Equal(t, test.stdout, stdoutLines)
	if len(test.outputs) > 0 {
		assert.Equal(t, test.outputs, output)
	}
	if test.outputs == nil {
		assert.Empty(t, output)
	} else {
		assert.Equal(t, test.outputs, output)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "buf.yaml")
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)
	return tmpDir
}

func githubNotFoundErr() error {
	return &gogithub.ErrorResponse{
		Response: &http.Response{
			StatusCode: http.StatusNotFound,
		},
	}
}

func v1Config(name string) string {
	return fmt.Sprintf(
		`
version: v1
name: %s
`,
		name,
	)
}
