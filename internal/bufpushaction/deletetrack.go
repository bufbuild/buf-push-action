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

	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/bufpkg/bufconfig"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/registry/v1alpha1/registryv1alpha1apiclient"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/rpc"
	"github.com/bufbuild/buf/private/pkg/storage/storageos"
	"github.com/sethvargo/go-githubactions"
)

func newDeleteTrackCommand(name string, builder appflag.Builder) *appcmd.Command {
	return &appcmd.Command{
		Use:   name,
		Short: "delete a track on BSR",
		Run:   builder.NewRunFunc(deleteTrack, actionInterceptor),
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
