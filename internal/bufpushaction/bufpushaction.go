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
	"github.com/bufbuild/buf/private/bufpkg/bufconfig"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/storage/storageos"
	"github.com/spf13/cobra"
)

const (
	bufTokenKey         = "BUF_TOKEN"
	githubTokenKey      = "GITHUB_TOKEN"
	githubRepositoryKey = "GITHUB_REPOSITORY"
)

var errNoTrackSupport = errors.New("The installed version of buf does not support setting the track. Please use buf v1.0.0 or newer.")

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
				Use:   "push <input> <track> <git-commit-hash>",
				Short: "push to BSR",
				Args:  cobra.ExactArgs(3),
				Run:   builder.NewRunFunc(runPush, interceptErrorForGithubAction),
			},
			{
				Use:   "delete-track <input> <track>",
				Short: "delete a track on BSR",
				Args:  cobra.ExactArgs(2),
				Run:   builder.NewRunFunc(runDeleteTrack, interceptErrorForGithubAction),
			},
		},
	}
}

func runPush(ctx context.Context, container appflag.Container) error {
	input := container.Arg(0)
	track := container.Arg(1)
	currentGitCommit := container.Arg(2)

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
	return push(ctx, input, track, moduleName, currentGitCommit, githubClient, container.Stdout(), &bufRunner{
		bufToken: container.Env(bufTokenKey),
		path:     container.Env("PATH"),
	})
}

func runDeleteTrack(ctx context.Context, container appflag.Container) error {
	input := container.Arg(0)
	track := container.Arg(1)
	bucket, err := storageos.NewProvider().NewReadWriteBucket(input)
	if err != nil {
		return fmt.Errorf("config file not found: %s", input)
	}
	moduleName, err := getNameFromConfigFile(ctx, bucket)
	if err != nil {
		return fmt.Errorf("name not found in  %s", input)
	}
	return deleteTrack(
		ctx,
		track,
		moduleName,
		container.Stdout(),
		&bufRunner{
			bufToken: container.Env(bufTokenKey),
			path:     container.Env("PATH"),
		},
	)
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

func getNameFromConfigFile(ctx context.Context, bucket storage.ReadBucket) (string, error) {
	config, err := bufconfig.GetConfigForBucket(ctx, bucket)
	if err != nil {
		return "", err
	}
	return config.ModuleIdentity.IdentityString(), nil
}

func deleteTrack(ctx context.Context, track, moduleName string, stdout io.Writer, runner commandRunner) error {
	if track == "main" {
		writeWorkflowNotice(stdout, "Skipping because the main track can not be deleted from BSR")
		return nil
	}
	if err := checkTrackSupport(ctx, runner); err != nil {
		return err
	}
	trackRef := fmt.Sprintf("%s:%s", moduleName, track)
	_, runStderr, err := runner.Run(ctx, "beta", "registry", "track", "delete", trackRef, "--force")
	if err != nil {
		return errors.New(runStderr)
	}
	if len(runStderr) > 0 {
		writeWorkflowNotice(stdout, runStderr)
	}
	return nil
}

func push(
	ctx context.Context,
	input string,
	track string,
	moduleName string,
	currentGitCommit string,
	githubClient github.Client,
	stdout io.Writer,
	runner commandRunner,
) error {
	// versions of buf prior to --track support emit "unknown flag: --track" when running `buf push --track foo --help`

	// make sure --track is supported
	if track != "main" {
		if err := checkTrackSupport(ctx, runner); err != nil {
			return err
		}
	}
	tags, err := getTags(ctx, track, moduleName, runner)
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
		status, err := githubClient.CompareCommits(ctx, tag, currentGitCommit)
		if err != nil {
			if github.IsNotFoundError(err) {
				continue
			}
			return err
		}
		switch status {
		case github.CompareCommitsStatusIdentical:
			writeWorkflowNotice(
				stdout,
				fmt.Sprintf("Skipping because the current git commit is already the head of track %s", track),
			)
			return nil
		case github.CompareCommitsStatusBehind:
			writeWorkflowNotice(
				stdout,
				fmt.Sprintf("Skipping because the current git commit is behind the head of track %s", track),
			)
			return nil
		case github.CompareCommitsStatusDiverged:
			writeWorkflowNotice(
				stdout,
				fmt.Sprintf("The current git commit is diverged from the head of track %s", track),
			)
		case github.CompareCommitsStatusAhead:
		default:
			return fmt.Errorf("unexpected status: %s", status)
		}
	}
	runStdout, runStderr, err := runner.Run(ctx, "push", "--track", track, "--tag", currentGitCommit, input)
	if err != nil {
		return errors.New(runStderr)
	}
	if len(runStderr) > 0 {
		writeWorkflowNotice(stdout, runStderr)
	}
	commit := runStdout
	if commit == "" {
		trackRef := fmt.Sprintf("%s:%s", moduleName, track)
		runStdout, runStderr, err = runner.Run(ctx, "beta", "registry", "commit", "get", trackRef, "--format", "json")
		if err != nil {
			return errors.New(runStderr)
		}
		var commitInfo struct {
			Commit string `json:"commit"`
		}
		if err := json.Unmarshal([]byte(runStdout), &commitInfo); err != nil {
			return errors.New("unable to parse commit info")
		}
		commit = commitInfo.Commit
	}
	setOutput(stdout, "commit", commit)
	setOutput(stdout, "commit_url", fmt.Sprintf("https://%s/tree/%s", moduleName, commit))
	return nil
}

func writeWorkflowNotice(stdout io.Writer, message string) {
	fmt.Fprintf(stdout, "::notice::%s\n", message)
}

func setOutput(stdout io.Writer, name, value string) {
	fmt.Fprintf(stdout, "::set-output name=%s::%s\n", name, value)
}

func getTags(ctx context.Context, track, moduleName string, bufRunner commandRunner) ([]string, error) {
	trackRef := fmt.Sprintf("%s:%s", moduleName, track)
	stdout, stderr, err := bufRunner.Run(ctx, "beta", "registry", "commit", "get", trackRef, "--format", "json")
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

func checkTrackSupport(ctx context.Context, bufRunner commandRunner) error {
	_, stderr, err := bufRunner.Run(ctx, "push", "--track", "anytrack", "--help")
	if err != nil {
		if strings.Contains(stderr, "unknown flag: --track") {
			return errNoTrackSupport
		}
		return err
	}
	return nil
}
