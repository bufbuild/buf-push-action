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
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/bufpkg/bufconfig"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/command"
	"github.com/bufbuild/buf/private/pkg/rpc"
	"github.com/bufbuild/buf/private/pkg/storage/storageos"
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

// Main is the entrypoint to the buf CLI.
func Main(name string) {
	appcmd.Main(context.Background(), NewRootCommand(name))
}

func NewRootCommand(name string) *appcmd.Command {
	builder := appflag.NewBuilder(name, appflag.BuilderWithTimeout(120*time.Second))
	return &appcmd.Command{
		Use:   name,
		Short: "helper for the GitHub Action buf-push-action",
		SubCommands: []*appcmd.Command{
			{
				Use:   "push <input> <track> <git-commit-hash> <default-branch> <ref-name>",
				Short: "push to BSR",
				Run:   builder.NewRunFunc(push, actionInterceptor, githubClientInterceptor),
			},
			{
				Use:   "delete-track",
				Short: "delete a track on BSR",
				Run:   builder.NewRunFunc(deleteTrack, actionInterceptor),
			},
		},
	}
}

func deleteTrack(ctx context.Context, container appflag.Container) error {
	action, ok := ctx.Value(actionContextKey).(*githubactions.Action)
	if !ok {
		return errors.New("action not found in context")
	}
	registryProvider, ok := ctx.Value(registryProviderContextKey).(registryv1alpha1apiclient.Provider)
	if !ok {
		return errors.New("registry provider not found in context")
	}
	input := action.GetInput(inputInputID)
	bucket, err := storageos.NewProvider().NewReadWriteBucket(input)
	if err != nil {
		return err
	}
	config, err := bufconfig.GetConfigForBucket(ctx, bucket)
	if err != nil {
		return err
	}
	if config.ModuleIdentity == nil {
		return errors.New("module identity not found in config")
	}
	moduleReference, err := bufmoduleref.ModuleReferenceForString(config.ModuleIdentity.IdentityString())
	if err != nil {
		return err
	}
	track := action.GetInput(trackInputID)
	if track == "" {
		return fmt.Errorf("track not provided")
	}
	defaultBranch := action.GetInput(defaultBranchInputID)
	if defaultBranch == "" {
		return fmt.Errorf("default_branch not provided")
	}
	refName := container.Env(githubRefNameKey)
	track = resolveTrack(track, defaultBranch, refName)
	if track == "main" {
		action.Noticef("Skipping because the main track can not be deleted from BSR")
		return nil
	}
	repositoryTrackService, err := registryProvider.NewRepositoryTrackService(ctx, moduleReference.Remote())
	if err != nil {
		return err
	}
	owner := moduleReference.Owner()
	repository := moduleReference.Repository()
	if err := repositoryTrackService.DeleteRepositoryTrackByName(
		ctx,
		owner,
		repository,
		track,
	); err != nil {
		if rpc.GetErrorCode(err) == rpc.ErrorCodeNotFound {
			return bufcli.NewModuleReferenceNotFoundError(moduleReference)
		}
		return err
	}
	return nil
}

func push(ctx context.Context, container appflag.Container) error {
	action, ok := ctx.Value(actionContextKey).(*githubactions.Action)
	if !ok {
		return errors.New("action not found in context")
	}
	input := action.GetInput(inputInputID)
	storageosProvider := bufcli.NewStorageosProvider(false)
	runner := command.NewRunner()
	module, moduleIdentity, err := bufcli.ReadModuleWithWorkspacesDisabled(
		ctx,
		container,
		storageosProvider,
		runner,
		input,
	)
	if err != nil {
		return err
	}
	protoModule, err := bufmodule.ModuleToProtoModule(ctx, module)
	if err != nil {
		return err
	}
	track := action.GetInput(trackInputID)
	if track == "" {
		return fmt.Errorf("track not provided")
	}
	defaultBranch := action.GetInput(defaultBranchInputID)
	if defaultBranch == "" {
		return fmt.Errorf("default_branch not provided")
	}
	refName := container.Env(githubRefNameKey)
	// Error when track is main and not overridden but the default branch is not main.
	// This is for situations where the default branch is something like master and there
	// is also a main branch. It prevents the main track from having commits from multiple git branches.
	if defaultBranch != "main" && track == "main" && track == refName {
		return errors.New("cannot push to main track from a non-default branch")
	}
	track = resolveTrack(track, defaultBranch, refName)
	registryProvider, ok := ctx.Value(registryProviderContextKey).(registryv1alpha1apiclient.Provider)
	if !ok {
		return errors.New("registry provider not found in context")
	}
	tags, err := getTags(ctx, registryProvider, moduleIdentity, track)
	if err != nil {
		return err
	}
	githubClient, ok := ctx.Value(githubClientContextKey).(github.Client)
	if !ok {
		return errors.New("github client not found in context")
	}
	currentGitCommit := container.Env(githubSHAKey)
	if currentGitCommit == "" {
		return errors.New("current git commit not found in environment")
	}
	for _, tag := range tags {
		var status github.CompareCommitsStatus
		status, err = githubClient.CompareCommits(ctx, tag, currentGitCommit)
		if err != nil {
			if github.IsNotFoundError(err) {
				continue
			}
			return err
		}
		switch status {
		case github.CompareCommitsStatusIdentical:
			action.Noticef("Skipping because the current git commit is already the head of track %s", track)
			return nil
		case github.CompareCommitsStatusBehind:
			action.Noticef("Skipping because the current git commit is behind the head of track %s", track)
			return nil
		case github.CompareCommitsStatusDiverged:
			action.Noticef("The current git commit is diverged from the head of track %s", track)
		case github.CompareCommitsStatusAhead:
		default:
			return fmt.Errorf("unexpected status: %s", status)
		}
	}
	remote := moduleIdentity.Remote()
	pushService, err := registryProvider.NewPushService(ctx, remote)
	if err != nil {
		return err
	}
	localModulePin, err := pushService.Push(
		ctx,
		moduleIdentity.Owner(),
		moduleIdentity.Repository(),
		"",
		protoModule,
		[]string{currentGitCommit},
		[]string{track},
	)
	if err != nil {
		if rpc.GetErrorCode(err) == rpc.ErrorCodeAlreadyExists {
			action.Noticef("The latest commit has the same content; not creating a new commit.")
		} else {
			return err
		}
	}

	action.SetOutput(commitOutputID, localModulePin.Commit)
	action.SetOutput(commitURLOutputID, fmt.Sprintf(
		"https://%s/tree/%s",
		moduleIdentity.IdentityString(),
		localModulePin.Commit,
	))
	return nil
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

func getTags(
	ctx context.Context,
	registryProvider registryv1alpha1apiclient.Provider,
	moduleIdentity bufmoduleref.ModuleIdentity,
	track string,
) ([]string, error) {
	repositoryCommitService, err := registryProvider.NewRepositoryCommitService(ctx, moduleIdentity.Remote())
	if err != nil {
		return nil, err
	}
	repositoryCommit, err := repositoryCommitService.GetRepositoryCommitByReference(
		ctx,
		moduleIdentity.Owner(),
		moduleIdentity.Repository(),
		track,
	)
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(repositoryCommit.Tags))
	for _, tag := range repositoryCommit.Tags {
		tagName := tag.Name
		if len(tagName) != 40 {
			continue
		}
		if _, err := hex.DecodeString(tagName); err != nil {
			continue
		}
		tags = append(tags, tagName)
	}
	return tags, nil
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
