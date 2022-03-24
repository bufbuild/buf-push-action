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
	"strings"
	"time"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/sethvargo/go-githubactions"
)

const (
	bufTokenKey         = "BUF_TOKEN"
	githubRepositoryKey = "GITHUB_REPOSITORY"
	githubRefNameKey    = "GITHUB_REF_NAME"
	githubSHAKey        = "GITHUB_SHA"
)

const (
	bufTokenInputID      = "buf_token"
	defaultBranchInputID = "default_branch"
	githubTokenInputID   = "github_token"
	inputInputID         = "input"
	trackInputID         = "track"
	commitOutputID       = "commit"
	commitURLOutputID    = "commit_url"
)

type contextKey int

const (
	actionContextKey contextKey = iota + 1
	registryProviderContextKey
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
		SubCommands: []*appcmd.Command{
			newDeleteTrackCommand("delete-track", builder),
			newPushCommand("push", builder),
		},
	}
}

func actionInterceptor(
	next func(context.Context, appflag.Container) error,
) func(context.Context, appflag.Container) error {
	return func(ctx context.Context, container appflag.Container) (retErr error) {
		defer func() {
			if retErr != nil {
				retErr = fmt.Errorf("::error::%v", retErr)
			}
		}()
		action := newAction(container)
		bufToken := action.GetInput(bufTokenInputID)
		if bufToken == "" {
			return errors.New("a buf authentication token was not provided")
		}
		container = newContainerWithEnvOverrides(container, map[string]string{
			bufTokenKey: bufToken,
		})
		_, ok := ctx.Value(registryProviderContextKey).(registryv1alpha1apiclient.Provider)
		if !ok {
			registryProvider, err := bufcli.NewRegistryProvider(ctx, container)
			if err != nil {
				return err
			}
			ctx = context.WithValue(ctx, registryProviderContextKey, registryProvider)
		}
		ctx = context.WithValue(ctx, actionContextKey, action)
		return next(ctx, container)
	}
}

// githubClientInterceptor adds a github client to ctx.
func githubClientInterceptor(
	next func(context.Context, appflag.Container) error,
) func(context.Context, appflag.Container) error {
	return func(ctx context.Context, container appflag.Container) error {
		action, ok := ctx.Value(actionContextKey).(*githubactions.Action)
		if !ok {
			return errors.New("action not found in context")
		}
		githubToken := action.GetInput(githubTokenInputID)
		if githubToken == "" {
			return errors.New("a github authentication token was not provided")
		}
		githubRepository := container.Env(githubRepositoryKey)
		if githubRepository == "" {
			return errors.New("a github repository was not provided")
		}
		repoParts := strings.Split(githubRepository, "/")
		if len(repoParts) != 2 {
			return errors.New("a github repository was not provided in the format owner/repo")
		}
		githubClient, err := github.NewClient(ctx, githubToken, "buf-push-action", "", githubRepository)
		if err != nil {
			return err
		}
		if ctx.Value(githubClientContextKey) == nil {
			ctx = context.WithValue(ctx, githubClientContextKey, githubClient)
		}
		return next(ctx, container)
	}
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

// newAction returns an action using the container's env and stdout.
func newAction(container app.EnvStdoutContainer) *githubactions.Action {
	return githubactions.New(
		githubactions.WithGetenv(container.Env),
		githubactions.WithWriter(container.Stdout()),
	)
}
