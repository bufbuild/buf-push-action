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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteTrack(t *testing.T) {
	subCommand := "delete-track"

	t.Run("happy path", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
		})
	})

	t.Run("main track", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			track:      testMainTrack,
			stdout: []string{
				"::notice::Skipping because the main track can not be deleted from BSR",
			},
		})
	})

	t.Run("NewRepositoryTrackService returns an error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				newRepositoryTrackServiceErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})

	t.Run("DeleteRepositoryTrackByName returns a NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				deleteRepositoryTrackByNameErr: testNotFoundErr,
			},
			errMsg: `buf.build/foo/bar:non-main does not exist`,
		})
	})

	t.Run("DeleteRepositoryTrackByName returns a non-NotFound error", func(t *testing.T) {
		runCmdTest(t, cmdTest{
			subCommand: subCommand,
			provider: fakeRegistryProvider{
				deleteRepositoryTrackByNameErr: assert.AnError,
			},
			errMsg: assert.AnError.Error(),
		})
	})
}
