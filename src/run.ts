// Copyright 2020-2021 Buf Technologies, Inc.
//
// All rights reserved.

import * as core from '@actions/core';
import * as github from '@actions/github'
import * as io from '@actions/io';
import * as fs from 'fs';
import * as path from 'path';
import cp from 'child_process';
import { Error, isError } from './error';

// runnerTempEnvKey is the environment variable key
// used to access a temporary directory. Although
// undocumented in the Github Actions documentation,
// this can be found in the @actions/tools-cache module.
// https://github.com/actions/toolkit/blob/4bf916289e5e32bb7d1bd7f21842c3afeab3b25a/packages/tool-cache/src/tool-cache.ts#L701
const runnerTempEnvKey = 'RUNNER_TEMP'

export async function run(): Promise<void> {
    try {
        const result = await runPush();
        if (result !== null && isError(result)) {
            core.setFailed(result.message);
        }
    } catch (error) {
        // In case we ever fail to catch an error
        // in the call chain, we catch the error
        // and mark the build as a failure. The
        // user is otherwise prone to false positives.
        if (isError(error)) {
            core.setFailed(error.message);
            return;
        }
        core.setFailed('Internal error');
    }
}

// runPush runs the buf-push action, and returns
// a non-empty error if it fails.
async function runPush(): Promise<null|Error> {
    const authenticationToken = core.getInput('buf_token');
    if (authenticationToken === '') {
        return {
            message: 'a buf authentication token was not provided'
        };
    }
    const branch = core.getInput('branch');
    if (branch === '') {
        return {
            message: 'a repository branch was not provided'
        };
    }
    const commit = github.context.sha;
    if (commit === '') {
        return {
            message: 'the commit was not provided'
        };
    }
    const input = core.getInput('input');
    if (input === '') {
        return {
            message: 'an input was not provided'
        };
    }
    const binaryPath = await io.which('buf', true);
    if (binaryPath === '') {
        // TODO: Update this reference to a link once it's available.
        return {
            message: 'buf is not installed; please add the "bufbuild/setup-buf" step to your job'
        };
    }

    const tempDir = process.env[runnerTempEnvKey] ?? '';
    if (tempDir === '') {
        return {
            message: `expected ${runnerTempEnvKey} to be defined`
        };
    }

    // TODO: For now, we hard-code the 'buf.build' remote. This will
    // need to be refactored once we support federation between other
    // BSR remotes.
    const netrcPath = path.join(tempDir, '.netrc');
    fs.writeFileSync(netrcPath, `machine buf.build\npassword ${authenticationToken}`, { flag: 'w' });

    cp.execSync(
        `${binaryPath} beta push -t ${commit}`,
        {
            cwd: input,
            env: {
                NETRC: netrcPath,
            },
        },
    );

    return null;
}
