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
	"testing"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCommandRunnerRun struct {
	expectArgs []string
	stdout     string
	stderr     string
	err        error
}

type fakeCommandRunner struct {
	t    *testing.T
	runs []fakeCommandRunnerRun
}

func (f *fakeCommandRunner) Run(_ context.Context, args ...string) (stdout, stderr string, err error) {
	require.Truef(f.t, len(f.runs) > 0, "unexpected call to Run: %v", args)
	fake := f.runs[0]
	f.runs = f.runs[1:]
	assert.Equal(f.t, fake.expectArgs, args)
	return fake.stdout, fake.stderr, fake.err
}

type fakeCompareCommits struct {
	expectBase string
	expectHead string
	status     github.CompareCommitsStatus
	err        error
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
