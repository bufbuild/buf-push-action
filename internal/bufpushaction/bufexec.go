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
	"bytes"
	"context"

	"github.com/bufbuild/buf/private/pkg/command"
)

type commandRunner interface {
	Run(ctx context.Context, args ...string) (stdout, stderr string, err error)
}

type bufRunner struct {
	bufToken string
	path     string
}

func (b *bufRunner) Run(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	var so, se bytes.Buffer
	err = command.NewRunner().Run(
		ctx,
		"buf",
		command.RunWithEnv(map[string]string{
			"BUF_TOKEN":                  b.bufToken,
			"BUF_BETA_SUPPRESS_WARNINGS": "1",
			"PATH":                       b.path,
		}),
		command.RunWithStdout(&so),
		command.RunWithStderr(&se),
		command.RunWithArgs(args...),
	)
	return so.String(), se.String(), err
}
