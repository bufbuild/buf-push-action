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
	"encoding/json"
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
	testGitCommit1         = "fa1afe1cafefa1afe1cafefa1afe1cafefa1afe1"
	testGitCommit2         = "beefcafebeefcafebeefcafebeefcafebeefcafe"
	testBsrCommit          = "01234567890123456789012345678901"
	testModuleName         = "buf.build/foo/bar"
	testOwner              = "foo"
	testRepository         = "bar"
	testInput              = "path/to/proto"
	testMainTrack          = "main"
	testAddress            = "buf.build"
	testModuleMainTrack    = "buf.build/foo/bar:main"
	testNonMainTrack       = "non-main"
	testModuleNonMainTrack = "buf.build/foo/bar:non-main"
)

type cmdTest struct {
	subCommand string
	provider   fakeRegistryProvider
	config     string
	env        map[string]string
	errMsg     string
	stdout     []string
}

func runCmdTest(t *testing.T, test cmdTest) {
	var stdout, stderr bytes.Buffer
	test.provider.t = t
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
		"GITHUB_REF_NAME":      testMainTrack,
	}
	for k, v := range defaultEnv {
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}
	ctx := context.WithValue(context.Background(), registryProviderContextKey, &test.provider)
	container := app.NewContainer(env, nil, &stdout, &stderr, "test", test.subCommand)
	command := NewRootCommand("test")
	err := appcmd.Run(ctx, container, command)
	assert.Equal(
		t,
		strings.TrimSpace(strings.Join(test.stdout, "\n")),
		strings.TrimSpace(stdout.String()),
	)

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
}

func writeConfigFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "buf.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	return tmpDir
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
		provider:   fakeRegistryProvider{
			deleteRepositoryTrackByNameErr: assert.AnError,
		},
		errMsg: assert.AnError.Error(),
	})

	runCmdTest(t, cmdTest{
		subCommand: "delete-track",
		provider:   fakeRegistryProvider{
			deleteRepositoryTrackByNameErr: rpc.NewNotFoundError("an error"),
		},
		errMsg: `"buf.build/foo/bar" does not exist`,
	})
}

func TestPush(t *testing.T) {
	t.Run("re-push the current main track head", func(t *testing.T) {
		runPushTest(t, pushTest{
			track: testMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				getTagsRun(t, testModuleMainTrack, testGitCommit1),
			},
			compareCommitRuns: []fakeCompareCommits{
				{
					expectBase: testGitCommit1,
					expectHead: testGitCommit1,
					status:     github.CompareCommitsStatusIdentical,
				},
			},
			expectStdout: []string{
				"::notice::Skipping because the current git commit is already the head of track main",
			},
		})
	})

	t.Run("old buf version", func(t *testing.T) {
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(false),
			},
			errorAssertion: func(err error) {
				assert.Equal(t, errNoTrackSupport, err)
			},
		})
	})

	t.Run("re-push the current non-main track head", func(t *testing.T) {
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				getTagsRun(t, testModuleNonMainTrack, testGitCommit1),
			},
			compareCommitRuns: []fakeCompareCommits{
				compareCommitsRun(testGitCommit1, testGitCommit1, github.CompareCommitsStatusIdentical),
			},
			expectStdout: []string{
				"::notice::Skipping because the current git commit is already the head of track non-main",
			},
		})
	})

	t.Run("push a commit behind head", func(t *testing.T) {
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				getTagsRun(t, testModuleNonMainTrack, testGitCommit2),
			},
			compareCommitRuns: []fakeCompareCommits{
				compareCommitsRun(testGitCommit2, testGitCommit1, github.CompareCommitsStatusBehind),
			},
			expectStdout: []string{
				"::notice::Skipping because the current git commit is behind the head of track non-main",
			},
		})
	})

	t.Run("push a commit ahead of head", func(t *testing.T) {
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				getTagsRun(t, testModuleNonMainTrack, testGitCommit2),
				{
					expectArgs: []string{"push", "--track", testNonMainTrack, "--tag", testGitCommit1, testInput},
					stdout:     testBsrCommit,
				},
			},
			compareCommitRuns: []fakeCompareCommits{
				compareCommitsRun(testGitCommit2, testGitCommit1, github.CompareCommitsStatusAhead),
			},
			expectStdout: []string{
				fmt.Sprintf("::set-output name=commit::%s", testBsrCommit),
				fmt.Sprintf("::set-output name=commit_url::%s", bsrCommitURL(testModuleName, testBsrCommit)),
			},
		})
	})

	t.Run("custom default branch", func(t *testing.T) {
		runPushTest(t, pushTest{
			track:         testNonMainTrack,
			defaultBranch: testNonMainTrack,
			refName:       testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				getTagsRun(t, testModuleMainTrack, testGitCommit2),
				{
					expectArgs: []string{"push", "--track", testMainTrack, "--tag", testGitCommit1, testInput},
					stdout:     testBsrCommit,
				},
			},
			compareCommitRuns: []fakeCompareCommits{
				compareCommitsRun(testGitCommit2, testGitCommit1, github.CompareCommitsStatusAhead),
			},
			expectStdout: []string{
				fmt.Sprintf("::set-output name=commit::%s", testBsrCommit),
				fmt.Sprintf("::set-output name=commit_url::%s", bsrCommitURL(testModuleName, testBsrCommit)),
			},
		})
	})

	t.Run("custom default branch push to main", func(t *testing.T) {
		runPushTest(t, pushTest{
			track:         testMainTrack,
			defaultBranch: testNonMainTrack,
			refName:       testMainTrack,
			errorAssertion: func(err error) {
				assert.EqualError(t, err, "cannot push to main track from a non-default branch")
			},
		})
	})

	t.Run("skips non-git tags", func(t *testing.T) {
		shortTag := "some-random-tag"
		nonHexTag := strings.Repeat("g", 40)
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				getTagsRun(t, testModuleNonMainTrack, shortTag, nonHexTag),
				{
					expectArgs: []string{"push", "--track", testNonMainTrack, "--tag", testGitCommit1, testInput},
					stdout:     testBsrCommit,
				},
			},
			expectStdout: []string{
				fmt.Sprintf("::set-output name=commit::%s", testBsrCommit),
				fmt.Sprintf("::set-output name=commit_url::%s", bsrCommitURL(testModuleName, testBsrCommit)),
			},
		})
	})

	t.Run("bsr repository does not exist", func(t *testing.T) {
		repoNotFoundMessage := fmt.Sprintf("Failure: repository %q was not found", testModuleName)
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				{
					expectArgs: []string{
						"beta",
						"registry",
						"commit",
						"get",
						testModuleNonMainTrack,
						"--format",
						"json",
					},
					stderr: fmt.Sprintf("Failure: %q does not exist", testModuleName),
					err:    assert.AnError,
				},
				{
					expectArgs: []string{"push", "--track", testNonMainTrack, "--tag", testGitCommit1, testInput},
					stderr:     repoNotFoundMessage,
					err:        assert.AnError,
				},
			},
			errorAssertion: func(err error) {
				require.EqualError(t, err, repoNotFoundMessage)
			},
		})
	})

	t.Run("push commit with same digest as head", func(t *testing.T) {
		dupContentMessage := "The latest commit has the same content; not creating a new commit."
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				getTagsRun(t, testModuleNonMainTrack, testGitCommit2),
				{
					expectArgs: []string{"push", "--track", testNonMainTrack, "--tag", testGitCommit1, testInput},
					stderr:     dupContentMessage,
				},
				{
					expectArgs: []string{
						"beta",
						"registry",
						"commit",
						"get",
						testModuleNonMainTrack,
						"--format",
						"json",
					},
					stdout: buildCommitJSON(t, testBsrCommit),
				},
			},
			compareCommitRuns: []fakeCompareCommits{
				compareCommitsRun(testGitCommit2, testGitCommit1, github.CompareCommitsStatusAhead),
			},
			expectStdout: []string{
				fmt.Sprintf("::notice::%s", dupContentMessage),
				fmt.Sprintf("::set-output name=commit::%s", testBsrCommit),
				fmt.Sprintf("::set-output name=commit_url::%s", bsrCommitURL(testModuleName, testBsrCommit)),
			},
		})
	})

	t.Run("tagged git commit not found on github", func(t *testing.T) {
		goGithubNotFoundError := &gogithub.ErrorResponse{
			Response: &http.Response{
				StatusCode: http.StatusNotFound,
				Request:    &http.Request{},
			},
		}
		runPushTest(t, pushTest{
			track: testNonMainTrack,
			bufRuns: []fakeCommandRunnerRun{
				trackHelpRun(true),
				getTagsRun(t, testModuleNonMainTrack, testGitCommit2),
				{
					expectArgs: []string{"push", "--track", testNonMainTrack, "--tag", testGitCommit1, testInput},
					stdout:     testBsrCommit,
				},
			},
			compareCommitRuns: []fakeCompareCommits{
				{
					expectBase: testGitCommit2,
					expectHead: testGitCommit1,
					err:        goGithubNotFoundError,
				},
			},
			expectStdout: []string{
				fmt.Sprintf("::set-output name=commit::%s", testBsrCommit),
				fmt.Sprintf("::set-output name=commit_url::%s", bsrCommitURL(testModuleName, testBsrCommit)),
			},
		})
	})
}

type pushTest struct {
	track             string
	defaultBranch     string
	refName           string
	bufRuns           []fakeCommandRunnerRun
	compareCommitRuns []fakeCompareCommits
	expectStdout      []string
	errorAssertion    func(err error)
}

func runPushTest(t *testing.T, pt pushTest) {
	ctx := context.Background()
	var stdout bytes.Buffer
	githubClient := fakeGithubClient{
		t:                  t,
		fakeCompareCommits: pt.compareCommitRuns,
	}
	cmdRunner := fakeCommandRunner{
		t:    t,
		runs: pt.bufRuns,
	}
	defaultBranch := pt.defaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	refName := pt.refName
	if refName == "" {
		refName = "main"
	}
	err := push(
		ctx,
		testInput,
		pt.track,
		testModuleName,
		testGitCommit1,
		defaultBranch,
		refName,
		&githubClient,
		&stdout,
		&cmdRunner,
	)
	if pt.errorAssertion != nil {
		pt.errorAssertion(err)
	} else {
		assert.NoError(t, err)
	}
	expectStdout := strings.Join(pt.expectStdout, "\n")
	assert.Equal(t, expectStdout, strings.TrimSpace(stdout.String()), "stdout")
	assert.Empty(t, githubClient.fakeCompareCommits, "missed compareCommit expectations")
	assert.Empty(t, cmdRunner.runs, "missed bufRunner expectations")
}

type deleteTrackTest struct {
	track          string
	bufRuns        []fakeCommandRunnerRun
	expectStdout   []string
	errorAssertion func(err error)
}

func trackHelpRun(ok bool) fakeCommandRunnerRun {
	args := []string{"push", "--track", "anytrack", "--help"}
	if ok {
		return fakeCommandRunnerRun{
			expectArgs: args,
			stdout:     "fake usage...",
		}
	}
	return fakeCommandRunnerRun{
		expectArgs: args,
		stderr:     "fake usage...\nerror: unknown flag: --track",
		err:        assert.AnError,
	}
}

func getTagsRun(t *testing.T, trackName string, tags ...string) fakeCommandRunnerRun {
	return fakeCommandRunnerRun{
		expectArgs: []string{"beta", "registry", "commit", "get", trackName, "--format", "json"},
		stdout:     buildCommitJSON(t, "", tags...),
	}
}

func buildCommitJSON(t *testing.T, commit string, tags ...string) string {
	tagMaps := make([]map[string]interface{}, len(tags))
	for i, tag := range tags {
		tagMaps[i] = map[string]interface{}{
			"name": tag,
		}
	}
	data := map[string]interface{}{
		"commit": commit,
		"tags":   tagMaps,
	}
	output, err := json.Marshal(&data)
	require.NoError(t, err)
	return string(output)
}

func compareCommitsRun(base, head string, status github.CompareCommitsStatus) fakeCompareCommits {
	return fakeCompareCommits{
		expectBase: base,
		expectHead: head,
		status:     status,
	}
}

func bsrCommitURL(moduleName, commit string) string {
	return fmt.Sprintf("https://%s/tree/%s", moduleName, commit)
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
