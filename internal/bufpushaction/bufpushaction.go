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
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/bufpkg/bufapiclient"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/bufpkg/bufrpc"
	"github.com/bufbuild/buf/private/bufpkg/buftransport"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/command"
	"github.com/bufbuild/buf/private/pkg/rpc/rpcauth"
	"github.com/spf13/cobra"
)

// action input and output IDs
const (
	commitOutputID    = "commit"
	commitURLOutputID = "commit_url"
)

// environment variable keys
const (
	bufTokenKey         = "BUF_TOKEN"
	githubTokenKey      = "GITHUB_TOKEN"
	githubRepositoryKey = "GITHUB_REPOSITORY"
	githubAPIURLKey     = "GITHUB_API_URL"
)

type contextKey int

// context keys
const (
	registryProviderContextKey contextKey = iota + 1
	githubClientContextKey
)

// githubClient is implemented by *github.Client
type githubClient interface {
	CompareCommits(ctx context.Context, base, head string) (github.CompareCommitsStatus, error)
}

// Main is the entrypoint to the buf CLI.
func Main(name string) {
	appcmd.Main(context.Background(), newRootCommand(name))
}

func newRootCommand(name string) *appcmd.Command {
	builder := appflag.NewBuilder(name, appflag.BuilderWithTimeout(120*time.Second))
	return &appcmd.Command{
		Use:   name,
		Short: "helper for the GitHub Action buf-push-action",
		SubCommands: []*appcmd.Command{
			{
				Use:   "push <input> <track> <git-commit-hash> <default-branch> <ref-name>",
				Short: "push to BSR",
				Args:  cobra.ExactArgs(5),
				Run:   builder.NewRunFunc(push, interceptErrorForGithubAction),
			},
			{
				Use:   "delete-track <input> <track> <default-branch> <ref-name>",
				Short: "delete a track on BSR",
				Args:  cobra.ExactArgs(4),
				Run:   builder.NewRunFunc(deleteTrack, interceptErrorForGithubAction),
			},
		},
	}
}

// commonArgs are the args that push and deleteTrack share
type commonArgs struct {
	track         string
	defaultBranch string
	refName       string
}

// resolveTrack returns track unless it is
//    1) set to ${{ github.ref_name }}
//      AND
//    2) equal to defaultBranch
// in which case it returns "main"
func (a *commonArgs) resolveTrack() string {
	if a.track == a.defaultBranch && a.track == a.refName {
		return bufmoduleref.MainTrack
	}
	return a.track
}

// commonSetup does the setup that is required for both push and deleteTrack
func commonSetup(
	ctx context.Context,
	container appflag.Container,
) (
	context.Context,
	*commonArgs,
	registryv1alpha1apiclient.Provider,
	bufmoduleref.ModuleIdentity,
	bufmodule.Module,
	error,
) {
	bufToken := container.Env(bufTokenKey)
	if bufToken == "" {
		return ctx, nil, nil, nil, nil, errors.New("buf_token is empty")
	}
	ctx = rpcauth.WithToken(ctx, bufToken)
	ctx = bufrpc.WithOutgoingCLIVersionHeader(ctx, bufcli.Version)
	registryProvider, err := newRegistryProvider(ctx, container)
	if err != nil {
		return ctx, nil, nil, nil, nil, err
	}
	input := container.Arg(0)
	if input == "" {
		return ctx, nil, nil, nil, nil, errors.New("input is empty")
	}
	track := container.Arg(1)
	if track == "" {
		return ctx, nil, nil, nil, nil, errors.New("track is empty")
	}
	defaultBranch := container.Arg(2)
	if defaultBranch == "" {
		return ctx, nil, nil, nil, nil, errors.New("default_branch is empty")
	}
	refName := container.Arg(3)
	if refName == "" {
		return ctx, nil, nil, nil, nil, errors.New("github.ref_name is empty")
	}
	module, moduleIdentity, err := bufcli.ReadModuleWithWorkspacesDisabled(
		ctx,
		container,
		bufcli.NewStorageosProvider(false),
		command.NewRunner(),
		input,
	)
	if err != nil {
		return ctx, nil, nil, nil, nil, err
	}
	return ctx, &commonArgs{
		track:         track,
		defaultBranch: defaultBranch,
		refName:       refName,
	}, registryProvider, moduleIdentity, module, nil
}

// interceptErrorForGithubAction intercepts errors and wraps them in formatting required for an error to be shown in
// the workflow results.
func interceptErrorForGithubAction(
	next func(context.Context, appflag.Container) error,
) func(context.Context, appflag.Container) error {
	return func(ctx context.Context, container appflag.Container) error {
		err := next(ctx, container)
		if err == nil {
			return nil
		}
		return fmt.Errorf("::error::%v", err)
	}
}

// newRegistryProvider returns a registry provider from the context if one is present or creates a provider.
func newRegistryProvider(
	ctx context.Context,
	container appflag.Container,
) (registryv1alpha1apiclient.Provider, error) {
	config, err := bufcli.NewConfig(container)
	if err != nil {
		return nil, err
	}
	var options []bufapiclient.RegistryProviderOption
	if buftransport.IsAPISubdomainEnabled(container) {
		options = append(options, bufapiclient.RegistryProviderWithAddressMapper(buftransport.PrependAPISubdomain))
	}
	provider, err := bufapiclient.NewRegistryProvider(ctx, container.Logger(), config.TLS, options...)
	if err != nil {
		return nil, err
	}
	// So tests can inject a provider
	if value, ok := ctx.Value(registryProviderContextKey).(registryv1alpha1apiclient.Provider); ok {
		provider = value
	}
	return provider, nil
}

// writeNotice writes a notice for a GitHub Action.
func writeNotice(w io.Writer, message string) {
	fmt.Fprintf(w, "::notice::%s\n", message)
}

// setOutput sets the output of a GitHub Action.
func setOutput(stdout io.Writer, name, value string) {
	fmt.Fprintf(stdout, "::set-output name=%s::%s\n", name, value)
}
