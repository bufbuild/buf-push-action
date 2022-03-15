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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/bufbuild/buf-push-action/internal/pkg/github"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/storage/storageos"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v2"
)

const (
	bufTokenKey         = "BUF_TOKEN"
	githubTokenKey      = "GITHUB_TOKEN"
	githubRepositoryKey = "GITHUB_REPOSITORY"
)

// Main is the entrypoint to the buf CLI.
func Main(name string) {
	appcmd.Main(context.Background(), NewRootCommand(name))
}

func NewRootCommand(name string) *appcmd.Command {
	builder := appflag.NewBuilder(name, appflag.BuilderWithTimeout(120*time.Second))
	return &appcmd.Command{
		Use:   name + " <input> <track> <git-commit-hash>",
		Short: "helper for the GitHub Action buf-push-action",
		Args:  cobra.ExactArgs(3),
		Run:   builder.NewRunFunc(run, interceptErrorForGithubAction),
	}
}

func run(ctx context.Context, container appflag.Container) error {
	input := container.Arg(0)
	track := container.Arg(1)
	currentGitCommit := container.Arg(2)

	// exit early if buf or BUF_TOKEN isn't found
	if _, err := exec.LookPath("buf"); err != nil {
		return errors.New(`buf is not installed; please add the "bufbuild/buf-setup-action" step to your job found at https://github.com/bufbuild/buf-setup-action`)
	}
	if container.Env(bufTokenKey) == "" {
		return errors.New("a buf authentication token was not provided")
	}
	githubRepository := container.Env(githubRepositoryKey)
	if githubRepository == "" {
		return errors.New("a github repository was not provided")
	}
	repoParts := strings.Split(githubRepository, "/")
	if len(repoParts) != 2 {
		return errors.New("a github repository was not provided in the format owner/repo")
	}
	githubClient, err := github.NewClient(ctx, container.Env(githubTokenKey), "buf-push-action", "", githubRepository)
	if err != nil {
		return err
	}
	bucket, err := storageos.NewProvider().NewReadWriteBucket(input)
	if err != nil {
		return fmt.Errorf("config file not found: %s", input)
	}
	moduleName, err := getNameFromConfigFile(ctx, bucket)
	if err != nil {
		return fmt.Errorf("name not found in  %s", input)
	}
	return (&pusher{
		input:            input,
		track:            track,
		stdout:           container.Stdout(),
		stderr:           container.Stderr(),
		currentGitCommit: currentGitCommit,
		moduleName:       moduleName,
		githubClient:     githubClient,
		bufRunner: &bufRunner{
			bufToken: container.Env(bufTokenKey),
			path:     container.Env("PATH"),
		},
	}).push(ctx)
}

// interceptErrorForGithubAction intercepts errors and wraps them in formatting required for an error to be shown in
// the workflow results.
func interceptErrorForGithubAction(
	next func(context.Context, appflag.Container) error,
) func(context.Context, appflag.Container) error {
	return func(ctx context.Context, container appflag.Container) error {
		err := next(ctx, container)
		if err == nil {
			return nil
		}
		return fmt.Errorf("::error::%v", err)
	}
}

func getNameFromConfigFile(ctx context.Context, bucket storage.ReadBucket) (_ string, retErr error) {
	file, err := bucket.Get(ctx, "buf.yaml")
	if err != nil {
		return "", err
	}
	defer func() {
		retErr = multierr.Append(retErr, file.Close())
	}()
	name := struct {
		Name string `yaml:"name"`
	}{}
	err = yaml.NewDecoder(file).Decode(&name)
	if err != nil {
		return "", err
	}
	return name.Name, nil
}

type pusher struct {
	input            string
	track            string
	stdout           io.Writer
	stderr           io.Writer
	currentGitCommit string
	moduleName       string
	githubClient     github.Client
	bufRunner        commandRunner
}

var errNoTrackSupport = errors.New("The installed version of buf does not support setting the track. Please use buf v1.0.0 or newer.")

func (p *pusher) push(ctx context.Context) error {
	// versions of buf prior to --track support emit "unknown flag: --track" when running `buf push --track foo --help`

	// make sure --track is supported
	if p.track != "main" {
		_, stderr, err := p.bufRunner.Run(ctx, "push", "--track", p.track, "--help")
		if err != nil {
			if strings.Contains(stderr, "unknown flag: --track") {
				return errNoTrackSupport
			}
			return err
		}
	}
	tags, err := p.getTags(ctx)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if len(tag) != 40 {
			continue
		}
		if _, err := hex.DecodeString(tag); err != nil {
			continue
		}
		status, err := p.githubClient.CompareCommits(ctx, tag, p.currentGitCommit)
		if err != nil {
			if github.IsNotFoundError(err) {
				continue
			}
			return err
		}
		switch status {
		case github.CompareCommitsStatusIdentical:
			p.notice(fmt.Sprintf("Skipping because the current git commit is already the head of track %s", p.track))
			return nil
		case github.CompareCommitsStatusBehind:
			p.notice(fmt.Sprintf("Skipping because the current git commit is behind the head of track %s", p.track))
			return nil
		case github.CompareCommitsStatusDiverged, github.CompareCommitsStatusAhead:
		default:
			return fmt.Errorf("unexpected status: %s", status)
		}
	}
	stdout, stderr, err := p.bufRunner.Run(ctx, "push", "--track", p.track, "--tag", p.currentGitCommit, p.input)
	if err != nil {
		return errors.New(stderr)
	}
	if len(stderr) > 0 {
		p.notice(stderr)
	}
	commit := stdout
	if commit == "" {
		trackRef := fmt.Sprintf("%s:%s", p.moduleName, p.track)
		stdout, stderr, err = p.bufRunner.Run(ctx, "beta", "registry", "commit", "get", trackRef, "--format", "json")
		if err != nil {
			return errors.New(stderr)
		}
		var commitInfo struct {
			Commit string `json:"commit"`
		}
		if err := json.Unmarshal([]byte(stdout), &commitInfo); err != nil {
			return errors.New("unable to parse commit info")
		}
		commit = commitInfo.Commit
	}
	p.setOutput("commit", commit)
	p.setOutput("commit_url", fmt.Sprintf("https://%s/tree/%s", p.moduleName, commit))
	return nil
}

func (p *pusher) notice(message string) {
	fmt.Fprintln(p.stdout, workflowNotice(message))
}

func workflowNotice(message string) string {
	return fmt.Sprintf("::notice::%s", message)
}

func (p *pusher) setOutput(name, value string) {
	fmt.Fprintln(p.stdout, workflowOutput(name, value))
}

func workflowOutput(name, value string) string {
	return fmt.Sprintf("::set-output name=%s::%s", name, value)
}

func (p *pusher) getTags(ctx context.Context) ([]string, error) {
	trackRef := fmt.Sprintf("%s:%s", p.moduleName, p.track)
	stdout, stderr, err := p.bufRunner.Run(ctx, "beta", "registry", "commit", "get", trackRef, "--format", "json")
	if err != nil {
		if strings.Contains(stderr, "does not exist") {
			return nil, nil
		}
		return nil, err
	}
	var result struct {
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}
	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		return nil, err
	}
	tags := make([]string, len(result.Tags))
	for i, tag := range result.Tags {
		tags[i] = tag.Name
	}
	return tags, nil
}
