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
	"github.com/bufbuild/buf/private/bufpkg/bufrpc"
	"github.com/bufbuild/buf/private/bufpkg/buftransport"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/rpc/rpcauth"
)

// environment variable keys
const (
	githubRepositoryKey = "GITHUB_REPOSITORY"
	githubRefNameKey    = "GITHUB_REF_NAME"
	githubRefTypeKey    = "GITHUB_REF_TYPE"
	githubSHAKey        = "GITHUB_SHA"
	githubEventNameKey  = "GITHUB_EVENT_NAME"
	bufTokenInput       = "INPUT_BUF_TOKEN"
	inputInput          = "INPUT_INPUT"
	defaultBranchInput  = "INPUT_DEFAULT_BRANCH"
	trackInput          = "INPUT_TRACK"
	githubTokenInput    = "INPUT_GITHUB_TOKEN"
)

// action input and output IDs
const (
	commitOutputID    = "commit"
	commitURLOutputID = "commit_url"
)

// constants used in the actions API
const (
	githubEventTypeDelete           = "delete"
	githubEventTypePush             = "push"
	githubEventTypeWorkflowDispatch = "workflow_dispatch"
	githubRefTypeBranch             = "branch"
)

type contextKey int

// context keys
const (
	registryProviderContextKey contextKey = iota + 1
	githubClientContextKey
)

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
		Run:   builder.NewRunFunc(run),
	}
}

func run(ctx context.Context, container appflag.Container) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("::error::%v", retErr)
		}
	}()
	bufToken := container.Env(bufTokenInput)
	if bufToken == "" {
		return errors.New("a buf authentication token was not provided")
	}
	ctx = bufrpc.WithOutgoingCLIVersionHeader(ctx, bufcli.Version)
	ctx = rpcauth.WithToken(ctx, bufToken)
	registryProvider, err := newRegistryProvider(ctx, container)
	if err != nil {
		return err
	}
	eventName := container.Env(githubEventNameKey)
	switch eventName {
	case "":
		return errors.New("a github event name was not provided")
	case githubEventTypeDelete:
		return deleteTrack(ctx, container, eventName, registryProvider)
	case githubEventTypePush, githubEventTypeWorkflowDispatch:
		return push(ctx, container, eventName, registryProvider)
	default:
		writeNotice(container.Stdout(), fmt.Sprintf("Skipping because %q events are not supported", eventName))
	}
	return nil
}

// newRegistryProvider returns a registry provider from the context if one is present or creates a provider.
func newRegistryProvider(ctx context.Context, container appflag.Container) (registryv1alpha1apiclient.Provider, error) {
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
	if value, ok := ctx.Value(registryProviderContextKey).(registryv1alpha1apiclient.Provider); ok {
		provider = value
	}
	return provider, nil
}

// resolveTrack returns track unless it is
//    1) set to ${{ github.ref_name }}
//      AND
//    2) equal to defaultBranch
// in which case it returns "main"
func resolveTrack(track, defaultBranch, refName string) string {
	if track == defaultBranch && (track == refName || refName == "") {
		return "main"
	}
	return track
}

// setOutput sets an output value for a GitHub Action.
func setOutput(w io.Writer, name, value string) {
	fmt.Fprintf(w, "::set-output name=%s::%s\n", name, value)
}

// writeNotice writes a notice for a GitHub Action.
func writeNotice(w io.Writer, message string) {
	fmt.Fprintf(w, "::notice::%s\n", message)
}
