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

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/command"
	"github.com/bufbuild/buf/private/pkg/rpc"
	"github.com/sethvargo/go-githubactions"
)

func newPushCommand(name string, builder appflag.Builder) *appcmd.Command {
	return &appcmd.Command{
		Use:   name,
		Short: "push to BSR",
		Run:   builder.NewRunFunc(push, actionInterceptor, githubClientInterceptor),
	}
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
