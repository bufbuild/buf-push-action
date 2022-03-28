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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	gogithub "github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/assert"
)

func TestPush(t *testing.T) {
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

	t.Run("module has no files", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			input:      "./testdata/empty_module",
			errMsg:     "module has no files",
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

	t.Run("GetRepositoryCommitByReference returns a non-rpc error", func(t *testing.T) {
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

	t.Run("GetRepositoryCommitByReference returns a FailedPrecondition error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				getRepositoryCommitByReferenceErr: testFailedPreconditionErr,
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
