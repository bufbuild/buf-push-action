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
	"strings"
	"testing"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/rpc"
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
	testNotFoundErr           = rpc.NewNotFoundError("testNotFoundErr")
	testAlreadyExistsErr      = rpc.NewAlreadyExistsError("testAlreadyExistsErr")
	testFailedPreconditionErr = rpc.NewFailedPreconditionError("testFailedPreconditionErr")
)

type cmdTest struct {
	subCommand       string
	provider         fakeRegistryProvider
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

func TestCommonSetup(t *testing.T) {
	runCommonSetup := func(
		ctx context.Context,
		env map[string]string,
		args ...string,
	) (
		context.Context,
		*commonArgs,
		registryv1alpha1apiclient.Provider,
		bufmoduleref.ModuleIdentity,
		bufmodule.Module,
		error,
	) {
		if env == nil {
			env = make(map[string]string)
		}
		if _, ok := env[bufTokenKey]; !ok {
			env[bufTokenKey] = "buf-token"
		}
		cmdName := "test"
		builder := appflag.NewBuilder(cmdName)
		args = append([]string{cmdName}, args...)
		var gotCtx context.Context
		var gotArgs *commonArgs
		var gotProvider registryv1alpha1apiclient.Provider
		var gotModuleIdentity bufmoduleref.ModuleIdentity
		var gotModule bufmodule.Module
		var gotErr error
		appContainer := app.NewContainer(env, nil, nil, nil, args...)
		command := appcmd.Command{
			Use: cmdName,
			Run: builder.NewRunFunc(func(ctx context.Context, container appflag.Container) error {
				gotCtx, gotArgs, gotProvider, gotModuleIdentity, gotModule, gotErr = commonSetup(ctx, container)
				return nil
			}),
		}
		err := appcmd.Run(context.Background(), appContainer, &command)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		return gotCtx, gotArgs, gotProvider, gotModuleIdentity, gotModule, gotErr
	}

	assertCommonSetupError := func(
		t *testing.T,
		errorMessage string,
		env map[string]string,
		args ...string,
	) {
		ctx := context.Background()
		_, _, _, _, _, err := runCommonSetup(ctx, env, args...)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), errorMessage)
		}
	}

	t.Run("happy path", func(t *testing.T) {
		const (
			// This matches authenticationHeader in rpcauth
			authenticationHeader = "Authorization"
			// This matches cliVersionHeaderName in bufrpc
			cliVersionHeaderName = "buf-version"
		)
		env := map[string]string{
			bufTokenKey: "buf-token",
		}
		inputArgs := []string{"testdata/success", testMainTrack, "default_branch", testNonMainTrack}
		ctx, args, _, identity, _, err := runCommonSetup(context.Background(), env, inputArgs...)
		require.NoError(t, err)
		assert.Equal(t, testMainTrack, args.track)
		assert.Equal(t, "default_branch", args.defaultBranch)
		assert.Equal(t, testNonMainTrack, args.refName)
		assert.Equal(t, "buf.build/foo/bar", identity.IdentityString())
		assert.Equal(t, "Bearer buf-token", rpc.GetOutgoingHeader(ctx, authenticationHeader))
		assert.Equal(t, bufcli.Version, rpc.GetOutgoingHeader(ctx, cliVersionHeaderName))
	})

	t.Run("input is empty", func(t *testing.T) {
		assertCommonSetupError(t, "input is empty", nil, "", testMainTrack, "main", testNonMainTrack)
	})

	t.Run("track is empty", func(t *testing.T) {
		assertCommonSetupError(t, "track is empty", nil, "testdata/success", "", "main", testNonMainTrack)
	})

	t.Run("default_branch is empty", func(t *testing.T) {
		assertCommonSetupError(t, "default_branch is empty", nil, "testdata/success", testMainTrack, "", testNonMainTrack)
	})

	t.Run("github.ref_name is empty", func(t *testing.T) {
		assertCommonSetupError(t, "github.ref_name is empty", nil, "testdata/success", testMainTrack, "main", "")
	})

	t.Run("empty buf_token", func(t *testing.T) {
		env := map[string]string{
			bufTokenKey: "",
		}
		assertCommonSetupError(t, "buf_token is empty", env, "testdata/empty_module", testMainTrack, "main", testNonMainTrack)
	})

	t.Run("input path doesn't exist", func(t *testing.T) {
		errMsg := "path/does/not/exist: does not exist"
		assertCommonSetupError(t, errMsg, nil, "path/does/not/exist", testMainTrack, "main", testNonMainTrack)
	})

	t.Run("invalid buf.yaml", func(t *testing.T) {
		errMsg := "could not unmarshal as YAML"
		assertCommonSetupError(t, errMsg, nil, "./testdata/invalid_config", testMainTrack, "main", testNonMainTrack)
	})
}

func runCmdTest(t *testing.T, test cmdTest) {
	var stdout, stderr bytes.Buffer
	test.provider.t = t
	test.githubClient.t = t
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

// resolveTestString replaces testEmpty with "" and replaces "" with defaultValue.
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
