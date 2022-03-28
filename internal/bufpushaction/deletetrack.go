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

	"github.com/bufbuild/buf/private/bufpkg/bufmodule/bufmoduleref"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/rpc"
)

func deleteTrack(ctx context.Context, container appflag.Container) error {
	ctx, args, registryProvider, moduleIdentity, _, err := commonSetup(ctx, container)
	if err != nil {
		return err
	}
	track := args.resolveTrack()
	if track == bufmoduleref.MainTrack {
		writeNotice(container.Stdout(), "Skipping because the main track can not be deleted from BSR")
		return nil
	}
	repositoryTrackService, err := registryProvider.NewRepositoryTrackService(ctx, moduleIdentity.Remote())
	if err != nil {
		return err
	}
	if err := repositoryTrackService.DeleteRepositoryTrackByName(
		ctx,
		moduleIdentity.Owner(),
		moduleIdentity.Repository(),
		track,
	); err != nil {
		if rpc.GetErrorCode(err) == rpc.ErrorCodeNotFound {
			return fmt.Errorf("%s:%s does not exist", moduleIdentity.IdentityString(), track)
		}
		return err
	}
	return nil
}
