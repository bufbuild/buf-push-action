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
	"fmt"
	"testing"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/gen/proto/api/buf/alpha/registry/v1alpha1/registryv1alpha1api"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	modulev1alpha1 "github.com/bufbuild/buf/private/gen/proto/go/buf/alpha/module/v1alpha1"
	registryv1alpha1 "github.com/bufbuild/buf/private/gen/proto/go/buf/alpha/registry/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRegistryProvider struct {
	registryv1alpha1apiclient.Provider
	registryv1alpha1api.RepositoryTrackService
	registryv1alpha1api.RepositoryTagService
	registryv1alpha1api.RepositoryService
	registryv1alpha1api.RepositoryCommitService
	registryv1alpha1api.PushService
	t                                 *testing.T
	address                           string
	ownerName                         string
	repositoryName                    string
	trackName                         string
	repositoryID                      string
	pushGitCommit                     string
	headTags                          []string
	deleteRepositoryTrackByNameErr    error
	getRepositoryCommitByReferenceErr error
	getRepositoryByFullNameErr        error
	pushErr                           error
	createRepositoryTagErr            error
	newRepositoryTrackServiceErr      error
	newRepositoryCommitServiceErr     error
	newPushServiceErr                 error
	newRepositoryServiceErr           error
	newRepositoryTagServiceErr        error
}

func (f *fakeRegistryProvider) DeleteRepositoryTrackByName(
	_ context.Context,
	ownerName string,
	repositoryName string,
	name string,
) error {
	wantOwnerName := f.ownerName
	if wantOwnerName == "" {
		wantOwnerName = testOwner
	}
	assert.Equal(f.t, wantOwnerName, ownerName)
	wantRepositoryName := f.repositoryName
	if wantRepositoryName == "" {
		wantRepositoryName = testRepository
	}
	assert.Equal(f.t, wantRepositoryName, repositoryName)
	wantTrackName := f.trackName
	if wantTrackName == "" {
		wantTrackName = testNonMainTrack
	}
	assert.Equal(f.t, wantTrackName, name)
	return f.deleteRepositoryTrackByNameErr
}

func (f *fakeRegistryProvider) NewRepositoryTrackService(
	_ context.Context,
	address string,
) (registryv1alpha1api.RepositoryTrackService, error) {
	wantAddress := f.address
	if wantAddress == "" {
		wantAddress = testAddress
	}
	assert.Equal(f.t, wantAddress, address)
	return f, f.newRepositoryTrackServiceErr
}

func (f *fakeRegistryProvider) GetRepositoryCommitByReference(
	_ context.Context,
	repositoryOwner string,
	repositoryName string,
	reference string,
) (*registryv1alpha1.RepositoryCommit, error) {
	wantRepositoryOwner := f.ownerName
	if wantRepositoryOwner == "" {
		wantRepositoryOwner = testOwner
	}
	assert.Equal(f.t, wantRepositoryOwner, repositoryOwner)
	wantRepositoryName := f.repositoryName
	if wantRepositoryName == "" {
		wantRepositoryName = testRepository
	}
	assert.Equal(f.t, wantRepositoryName, repositoryName)
	wantReference := f.trackName
	if wantReference == "" {
		wantReference = testNonMainTrack
	}
	assert.Equal(f.t, wantReference, reference)
	repositoryCommit := registryv1alpha1.RepositoryCommit{
		Name: testBsrCommit,
	}
	for _, tag := range f.headTags {
		repositoryCommit.Tags = append(repositoryCommit.Tags, &registryv1alpha1.RepositoryTag{
			Name: tag,
		})
	}
	return &repositoryCommit, f.getRepositoryCommitByReferenceErr
}

func (f *fakeRegistryProvider) NewRepositoryCommitService(
	_ context.Context,
	address string,
) (registryv1alpha1api.RepositoryCommitService, error) {
	wantAddress := f.address
	if wantAddress == "" {
		wantAddress = testAddress
	}
	assert.Equal(f.t, wantAddress, address)
	return f, f.newRepositoryCommitServiceErr
}

func (f *fakeRegistryProvider) Push(
	_ context.Context,
	owner string,
	repository string,
	branch string,
	module *modulev1alpha1.Module,
	tags []string,
	tracks []string,
) (*registryv1alpha1.LocalModulePin, error) {
	wantOwner := f.ownerName
	if wantOwner == "" {
		wantOwner = testOwner
	}
	assert.Equal(f.t, wantOwner, owner)
	wantRepository := f.repositoryName
	if wantRepository == "" {
		wantRepository = testRepository
	}
	assert.Equal(f.t, wantRepository, repository)
	assert.Equal(f.t, "", branch)
	assert.NotNil(f.t, module)
	pushGitCommit := f.pushGitCommit
	if pushGitCommit == "" {
		pushGitCommit = testGitCommit2
	}
	assert.Equal(f.t, []string{pushGitCommit}, tags)
	wantTrack := f.trackName
	if wantTrack == "" {
		wantTrack = testNonMainTrack
	}
	assert.Equal(f.t, []string{wantTrack}, tracks)
	return &registryv1alpha1.LocalModulePin{
		Commit: testBsrCommit,
	}, f.pushErr
}

func (f *fakeRegistryProvider) NewPushService(
	_ context.Context,
	address string,
) (registryv1alpha1api.PushService, error) {
	wantAddress := f.address
	if wantAddress == "" {
		wantAddress = testAddress
	}
	assert.Equal(f.t, wantAddress, address)
	return f, f.newPushServiceErr
}

func (f *fakeRegistryProvider) GetRepositoryByFullName(
	_ context.Context,
	fullName string,
) (*registryv1alpha1.Repository, *registryv1alpha1.RepositoryCounts, error) {
	wantOwner := f.ownerName
	if wantOwner == "" {
		wantOwner = testOwner
	}
	wantRepository := f.repositoryName
	if wantRepository == "" {
		wantRepository = testRepository
	}
	wantFullName := fmt.Sprintf("%s/%s", wantOwner, wantRepository)
	assert.Equal(f.t, wantFullName, fullName)
	repositoryID := f.repositoryID
	if repositoryID == "" {
		repositoryID = testRepositoryID
	}
	repository := registryv1alpha1.Repository{
		Id: repositoryID,
	}
	return &repository, nil, f.getRepositoryByFullNameErr
}

func (f *fakeRegistryProvider) NewRepositoryService(
	_ context.Context,
	address string,
) (registryv1alpha1api.RepositoryService, error) {
	wantAddress := f.address
	if wantAddress == "" {
		wantAddress = testAddress
	}
	assert.Equal(f.t, wantAddress, address)
	return f, f.newRepositoryServiceErr
}

func (f *fakeRegistryProvider) CreateRepositoryTag(
	_ context.Context,
	repositoryID string,
	name string,
	commitName string,
) (repositoryTag *registryv1alpha1.RepositoryTag, err error) {
	wantRepositoryID := f.repositoryID
	if wantRepositoryID == "" {
		wantRepositoryID = testRepositoryID
	}
	assert.Equal(f.t, wantRepositoryID, repositoryID)
	wantName := f.pushGitCommit
	if wantName == "" {
		wantName = testGitCommit2
	}
	assert.Equal(f.t, wantName, name)
	assert.Equal(f.t, testBsrCommit, commitName)
	return nil, f.createRepositoryTagErr
}

func (f *fakeRegistryProvider) NewRepositoryTagService(
	_ context.Context,
	address string,
) (registryv1alpha1api.RepositoryTagService, error) {
	wantAddress := f.address
	if wantAddress == "" {
		wantAddress = testAddress
	}
	assert.Equal(f.t, wantAddress, address)
	return f, f.newRepositoryTagServiceErr
}

type fakeGithubClient struct {
	t                  *testing.T
	fakeCompareCommits []fakeCompareCommits
}

func (f *fakeGithubClient) CompareCommits(_ context.Context, base, head string) (github.CompareCommitsStatus, error) {
	require.Truef(f.t, len(f.fakeCompareCommits) > 0, "unexpected call to Run: base: %q head: %q", base, head)
	fake := f.fakeCompareCommits[0]
	f.fakeCompareCommits = f.fakeCompareCommits[1:]
	assert.Equal(f.t, fake.expectBase, base)
	assert.Equal(f.t, fake.expectHead, head)
	return fake.status, fake.err
}

type fakeCompareCommits struct {
	expectBase string
	expectHead string
	status     github.CompareCommitsStatus
	err        error
}
