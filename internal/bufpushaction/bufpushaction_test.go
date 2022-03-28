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
	testMainTrack    = "main"
	testNonMainTrack = "non-main"
	testOwner        = "foo"
	testRepository   = "bar"
	testAddress      = "buf.build"
	testRepositoryID = "6b36a5d1-b845-4a97-885b-adbf52883819"
	testEmpty        = "_empty_value"
)

var (
	testNotFoundErr      = rpc.NewNotFoundError("testNotFoundErr")
	testAlreadyExistsErr = rpc.NewAlreadyExistsError("testAlreadyExistsErr")
)

type cmdTest struct {
	subCommand       string
	provider         fakeRegistryProvider
	config           string
	env              map[string]string
	errMsg           string
	stdout           []string
	outputs          map[string]string
	githubClient     fakeGithubClient
	input            string
	track            string
	defaultBranch    string
	refName          string
	currentGitCommit string
}

func TestPush2(t *testing.T) {
	successOutputs := map[string]string{
		commitOutputID:    testBsrCommit,
		commitURLOutputID: fmt.Sprintf("https://%s/tree/%s", testModuleName, testBsrCommit),
	}
	subCommand := "push"
	t.Run("happy path", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			outputs:    successOutputs,
		})
	})

	t.Run("input path doesn't exist", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      "path/does/not/exist",
			errMsg:     "path/does/not/exist: does not exist",
		})
	})

	t.Run("module has no files", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      writeConfigFile(t, v1Config(testModuleName)),
			errMsg:     "module has no files",
		})
	})

	t.Run("empty input", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      testEmpty,
			errMsg:     "input is empty",
		})
	})

	t.Run("empty track", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			track:      testEmpty,
			errMsg:     "track is empty",
		})
	})

	t.Run("empty default_branch", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand:    subCommand,
			defaultBranch: testEmpty,
			errMsg:        "default_branch is empty",
		})
	})

	t.Run("empty ref_name", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			refName:    testEmpty,
			errMsg:     "github.ref_name is empty",
		})
	})

	t.Run("empty current_git_commit", func(t *testing.T) {
		// This should never happen because it is set by GitHub Actions.
		runCmdTest(t, cmdTest{
			subCommand:       subCommand,
			currentGitCommit: testEmpty,
			errMsg:           "github.sha is empty",
		})
	})

	t.Run("empty github_token", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			env: map[string]string{
				githubTokenKey: "",
			},
			errMsg: "github_token is empty",
		})
	})

	t.Run("no BUF_TOKEN", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			env: map[string]string{
				bufTokenKey: "",
			},
			errMsg: "buf_token is empty",
		})
	})

	t.Run("no GITHUB_REPOSITORY", func(t *testing.T) {
		// This should never happen because it is set by GitHub Actions.
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			env: map[string]string{
				githubRepositoryKey: "",
			},
			errMsg: "GITHUB_REPOSITORY is empty",
		})
	})

	t.Run("unparseable GITHUB_API_URL", func(t *testing.T) {
		// This should never happen because it is set by GitHub Actions.
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			env: map[string]string{
				githubAPIURLKey: ":foo",
			},
			errMsg: `parse ":foo": missing protocol scheme`,
		})
	})

	t.Run("invalid GITHUB_REPOSITORY format", func(t *testing.T) {
		// This should never happen because it is set by GitHub Actions.
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			env: map[string]string{
				githubRepositoryKey: "invalid",
			},
			errMsg: "GITHUB_REPOSITORY is not in the format owner/repo",
		})
	})

	t.Run("pushing to main track from a non-default branch", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand:    subCommand,
			track:         testMainTrack,
			defaultBranch: testNonMainTrack,
			refName:       testMainTrack,
			errMsg:        "cannot push to main track from a non-default branch",
		})
	})

	t.Run("error from NewRepositoryCommitService", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				newRepositoryCommitServiceErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})

	t.Run("GetRepositoryCommitByReference returns non-NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				getRepositoryCommitByReferenceErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})

	t.Run("GetRepositoryCommitByReference returns NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				getRepositoryCommitByReferenceErr: testNotFoundErr,
			},
			outputs: successOutputs,
		})
	})

	t.Run("After GetRepositoryCommitByReference returns NotFound error", func(t *testing.T) {
		t.Run("Push returns an AlreadyExists error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					getRepositoryCommitByReferenceErr: testNotFoundErr,
					pushErr:                           testAlreadyExistsErr,
				},
				errMsg: testAlreadyExistsErr.Error(),
			})
		})
	})

	t.Run("Handles tags that aren't git commits", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				headTags: []string{"some", "other", "tags", strings.Repeat("z", 40)},
			},
			outputs: successOutputs,
		})
	})

	t.Run("CompareCommits returns NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			githubClient: fakeGithubClient{
				fakeCompareCommits: []fakeCompareCommits{
					{
						expectBase: testGitCommit1,
						expectHead: testGitCommit2,
						err: &gogithub.ErrorResponse{
							Response: &http.Response{
								StatusCode: http.StatusNotFound,
							},
						},
					},
				},
			},
			outputs: successOutputs,
		})
	})

	t.Run("CompareCommits returns a non-NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
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
	})

	t.Run("CompareCommits returns identical", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
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
	})

	t.Run("CompareCommits returns behind", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
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
	})

	t.Run("CompareCommits returns diverged", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
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
			outputs: successOutputs,
		})
	})

	t.Run("CompareCommits returns unknown status", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
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
	})

	t.Run("NewPushService returns an error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				newPushServiceErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})

	t.Run("Push returns an AlreadyExists error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				pushErr: testAlreadyExistsErr,
			},
			outputs: successOutputs,
		})
	})

	t.Run("Push returns a non-AlreadyExists error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				pushErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})

	t.Run("After Push returns an AlreadyExists error", func(t *testing.T) {
		t.Run("NewRepositoryService returns an error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                 testAlreadyExistsErr,
					newRepositoryServiceErr: assert.AnError,
				},
				errMsg: assert.AnError.Error(),
			})
		})

		t.Run("GetRepositoryByFullName returns a NotFound error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                    testAlreadyExistsErr,
					getRepositoryByFullNameErr: testNotFoundErr,
				},
				errMsg: `a repository named "buf.build/foo/bar" does not exist`,
			})
		})

		t.Run("GetRepositoryByFullName returns a non-NotFound error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                    testAlreadyExistsErr,
					getRepositoryByFullNameErr: assert.AnError,
				},
				errMsg: assert.AnError.Error(),
			})
		})

		t.Run("NewRepositoryTagService returns an error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                    testAlreadyExistsErr,
					newRepositoryTagServiceErr: assert.AnError,
				},
				errMsg: assert.AnError.Error(),
			})
		})

		t.Run("CreateRepositoryTag returns an error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                testAlreadyExistsErr,
					createRepositoryTagErr: assert.AnError,
				},
				errMsg: assert.AnError.Error(),
			})
		})

		t.Run("CreateRepositoryTag returns a NotFound error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                testAlreadyExistsErr,
					createRepositoryTagErr: testNotFoundErr,
				},
				errMsg: "buf.build/foo/bar:01234567890123456789012345678901 does not exist",
			})
		})

		t.Run("CreateRepositoryTag returns an AlreadyExists error", func(t *testing.T) {
			runCmdTest(t, cmdTest{
				subCommand: subCommand,
				provider: fakeRegistryProvider{
					pushErr:                testAlreadyExistsErr,
					createRepositoryTagErr: testAlreadyExistsErr,
				},
				errMsg: "buf.build/foo/bar:beefcafebeefcafebeefcafebeefcafebeefcafe already exists with different content",
			})
		})
	})
}

func TestDeleteTrack(t *testing.T) {
	subCommand := "delete-track"

	t.Run("happy path", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
		})
	})

	t.Run("input path doesn't exist", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      "path/does/not/exist",
			errMsg:     "path/does/not/exist: does not exist",
		})
	})

	t.Run("input path is empty dir", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      t.TempDir(),
			errMsg:     "module identity not found in config",
		})
	})

	t.Run("invalid buf.yaml", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      writeConfigFile(t, "invalid config"),
			errMsg:     "could not unmarshal as YAML",
		})
	})

	t.Run("empty input", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      testEmpty,
			errMsg:     "input is empty",
		})
	})

	t.Run("empty track", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			track:      testEmpty,
			errMsg:     "track is empty",
		})
	})

	t.Run("empty default_branch", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand:    subCommand,
			defaultBranch: testEmpty,
			errMsg:        "default_branch is empty",
		})
	})

	t.Run("empty ref_name", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			refName:    testEmpty,
			errMsg:     "github.ref_name is empty",
		})
	})

	t.Run("main track", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			track:      testMainTrack,
			stdout: []string{
				"::notice::Skipping because the main track can not be deleted from BSR",
			},
		})
	})

	t.Run("NewRepositoryTrackService returns an error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				newRepositoryTrackServiceErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})

	t.Run("DeleteRepositoryTrackByName returns a NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				deleteRepositoryTrackByNameErr: testNotFoundErr,
			},
			errMsg: `"buf.build/foo/bar:non-main" does not exist`,
		})
	})

	t.Run("DeleteRepositoryTrackByName returns a non-NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				deleteRepositoryTrackByNameErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})
}

func resolveTestString(s string, defaultValue string) string {
	switch s {
	case "":
		return defaultValue
	case testEmpty:
		return ""
	default:
		return s
	}
}

func runCmdTest(t *testing.T, test cmdTest) {
	var stdout, stderr bytes.Buffer
	test.provider.t = t
	test.githubClient.t = t
	test.config = resolveTestString(test.config, v1Config(testModuleName))
	test.input = resolveTestString(test.input, "./testdata/success")
	test.track = resolveTestString(test.track, testNonMainTrack)
	test.defaultBranch = resolveTestString(test.defaultBranch, testMainTrack)
	test.refName = resolveTestString(test.refName, testMainTrack)
	test.currentGitCommit = resolveTestString(test.currentGitCommit, testGitCommit2)
	env := test.env
	defaultEnv := map[string]string{
		bufTokenKey:         "buf-token",
		githubTokenKey:      "github-token",
		githubRepositoryKey: "github-owner/github-repo",
		githubAPIURLKey:     "https://api.github.com",
	}
	if env == nil {
		env = make(map[string]string)
	}
	for k, v := range defaultEnv {
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}
	if test.provider.headTags == nil {
		test.provider.headTags = []string{testGitCommit1}
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
	args := []string{
		"test",
		test.subCommand,
		test.input,
		test.track,
		test.defaultBranch,
		test.refName,
	}
	if test.subCommand == "push" {
		args = append(args, test.currentGitCommit)
	}
	container := app.NewContainer(env, nil, &stdout, &stderr, args...)
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
			delim := strings.Index(keyValue, "::")
			if delim == -1 {
				stdoutLines = append(stdoutLines, line)
				continue
			}
			key := keyValue[:delim]
			value := keyValue[delim+2:]
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

func v1Config(name string) string {
	return fmt.Sprintf(
		`
version: v1
name: %s
`,
		name,
	)
}
