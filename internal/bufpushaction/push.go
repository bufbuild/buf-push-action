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
	"net/http"
	"net/url"
	"strings"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/rpc"
)

func push(ctx context.Context, container appflag.Container) error {
	ctx, args, registryProvider, moduleIdentity, module, err := commonSetup(ctx, container)
	if err != nil {
		return err
	}
	currentGitCommit := container.Arg(4)
	if currentGitCommit == "" {
		return errors.New("github.sha is empty")
	}
	protoModule, err := bufmodule.ModuleToProtoModule(ctx, module)
	if err != nil {
		return err
	}
	// Error when track is main and not overridden but the default branch is not main.
	// This is for situations where the default branch is something like master and there
	// is also a main branch. It prevents the main track from having commits from multiple git branches.
	if args.defaultBranch != "main" && args.track == bufmoduleref.MainTrack && args.track == args.refName {
		return errors.New("cannot push to main track from a non-default branch")
	}
	track := args.resolveTrack()
	var tags []string
	repositoryCommitService, err := registryProvider.NewRepositoryCommitService(ctx, moduleIdentity.Remote())
	if err != nil {
		return err
	}
	repositoryCommit, err := repositoryCommitService.GetRepositoryCommitByReference(
		ctx,
		moduleIdentity.Owner(),
		moduleIdentity.Repository(),
		track,
	)
	if err != nil {
		switch rpc.GetErrorCode(err) {
		case rpc.ErrorCodeNotFound:
			// The track doesn't exist yet. Go ahead and push to create it.
		case rpc.ErrorCodeFailedPrecondition:
			// This could mean that the track exists but has no commits.
			// It could also mean that some other precondition is not met.
			// In either case we should go ahead and push. If the track exists but has no commits
			// then the push will add the first commit to the track. If some other precondition is not
			// met then the push will fail, and we can handle that error.
		default:
			return err
		}
		repositoryCommit = nil
	}
	if repositoryCommit != nil {
		tags = make([]string, 0, len(repositoryCommit.Tags))
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
	}

	ghClient, err := newGithubClient(ctx, container)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		var status github.CompareCommitsStatus
		status, err = ghClient.CompareCommits(ctx, tag, currentGitCommit)
		if err != nil {
			if github.IsResponseError(http.StatusNotFound, err) {
				continue
			}
			return err
		}
		switch status {
		case github.CompareCommitsStatusIdentical:
			writeNotice(
				container.Stdout(),
				fmt.Sprintf("Skipping because the current git commit is already the head of track %s", track),
			)
			return nil
		case github.CompareCommitsStatusBehind:
			writeNotice(
				container.Stdout(),
				fmt.Sprintf("Skipping because the current git commit is behind the head of track %s", track),
			)
			return nil
		case github.CompareCommitsStatusDiverged:
			writeNotice(
				container.Stdout(),
				fmt.Sprintf("The current git commit is diverged from the head of track %s", track),
			)
		case github.CompareCommitsStatusAhead:
		default:
			return fmt.Errorf("unexpected status: %s", status)
		}
	}
	pushService, err := registryProvider.NewPushService(ctx, moduleIdentity.Remote())
	if err != nil {
		return err
	}
	var commitName string
	owner := moduleIdentity.Owner()
	repository := moduleIdentity.Repository()
	localModulePin, err := pushService.Push(
		ctx,
		owner,
		repository,
		"",
		protoModule,
		[]string{currentGitCommit},
		[]string{track},
	)
	if err != nil {
		if rpc.GetErrorCode(err) != rpc.ErrorCodeAlreadyExists {
			return err
		}
		if repositoryCommit == nil {
			return err
		}
		commitName = repositoryCommit.Name
		if err := tagExistingCommit(ctx, registryProvider, moduleIdentity, currentGitCommit, commitName); err != nil {
			return err
		}
	} else {
		commitName = localModulePin.Commit
	}

	setOutput(container.Stdout(), commitOutputID, commitName)
	setOutput(container.Stdout(), commitURLOutputID, fmt.Sprintf(
		"https://%s/tree/%s",
		moduleIdentity.IdentityString(),
		commitName,
	))

	return nil
}

func tagExistingCommit(
	ctx context.Context,
	registryProvider registryv1alpha1apiclient.Provider,
	moduleIdentity bufmoduleref.ModuleIdentity,
	tagName string,
	reference string,
) error {
	repositoryService, err := registryProvider.NewRepositoryService(ctx, moduleIdentity.Remote())
	if err != nil {
		return err
	}
	repository, _, err := repositoryService.GetRepositoryByFullName(
		ctx,
		moduleIdentity.Owner()+"/"+moduleIdentity.Repository(),
	)
	if err != nil {
		if rpc.GetErrorCode(err) == rpc.ErrorCodeNotFound {
			return fmt.Errorf("a repository named %q does not exist", moduleIdentity.IdentityString())
		}
		return err
	}
	repositoryTagService, err := registryProvider.NewRepositoryTagService(ctx, moduleIdentity.Remote())
	if err != nil {
		return err
	}
	_, err = repositoryTagService.CreateRepositoryTag(ctx, repository.Id, tagName, reference)
	if err != nil {
		if rpc.GetErrorCode(err) == rpc.ErrorCodeNotFound {
			return fmt.Errorf("%s:%s does not exist", moduleIdentity.IdentityString(), reference)
		}
		if rpc.GetErrorCode(err) == rpc.ErrorCodeAlreadyExists {
			return fmt.Errorf("%s:%s already exists with different content", moduleIdentity.IdentityString(), tagName)
		}
		return err
	}
	return nil
}

// newGithubClient returns the github client from the context if one is present or creates a client.
func newGithubClient(ctx context.Context, container appflag.Container) (githubClient, error) {
	var githubAPIURL *url.URL
	if urlString := container.Env(githubAPIURLKey); urlString != "" {
		var err error
		githubAPIURL, err = url.Parse(urlString)
		if err != nil {
			return nil, err
		}
	}
	githubToken := container.Env(githubTokenKey)
	if githubToken == "" {
		return nil, errors.New("github_token is empty")
	}
	githubRepository := container.Env(githubRepositoryKey)
	if githubRepository == "" {
		return nil, errors.New("GITHUB_REPOSITORY is empty")
	}
	repoParts := strings.Split(githubRepository, "/")
	if len(repoParts) != 2 {
		return nil, errors.New("GITHUB_REPOSITORY is not in the format owner/repo")
	}
	var client githubClient
	client = github.NewClient(ctx, githubToken, "buf-push-action", repoParts[0], repoParts[1], githubAPIURL)
	// So tests can inject a client
	if value, ok := ctx.Value(githubClientContextKey).(githubClient); ok {
		client = value
	}
	return client, nil
}
