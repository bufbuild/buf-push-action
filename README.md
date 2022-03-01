# `buf-push-action`

This Action enables you to push [Buf modules][modules] to the [Buf Schema Registry][bsr] (BSR)
Pushed modules are created with the Git commit SHA as the module tag.

`buf-push-action` is frequently used alongside other Buf Actions, such as
[`buf-breaking-action`][buf-breaking] and [`buf-lint-action`][buf-lint].

## Usage

Here's an example usage of `buf-push-action`:

```yaml
on: push
jobs:
  push-module:
    runs-on: ubuntu-latest
    # only allow one concurrent push job per git branch to prevent race conditions
    concurrency: ${{ github.workflow }}-${{ github.ref_name }}
    steps:
      # Run `git checkout`
      - uses: actions/checkout@v2
      # Install the `buf` CLI
      - uses: bufbuild/buf-setup-action@v0.6.0
      # Push module to the BSR
      - uses: bufbuild/buf-push-action@v1
        id: push
        with:
          buf_token: ${{ secrets.BUF_TOKEN }}
          track: ${{ github.ref_name }}
      # Add a commit status with a link to the newly pushed module on BSR
      - name: post commit status to github
        env:
          GIHUB_TOKEN: ${{ github.token }}
        run: |
          gh api repos/{owner}/{repo}/statuses/${GITHUB_SHA} \
            -F "state=success" \
            -F "target_url=${{ steps.push.outputs.commit_url }}" \
            -F "description=pushed commit ${{ steps.push.outputs.commit }}" \
            -F "context=buf-push"
```

With this configuration, the `buf` CLI pushes the [configured module][buf-yaml] to the BSR upon
merge using a Buf API token to authenticate with the [Buf Schema Registry][bsr] (BSR).

For instructions on creating a BSR API token, see our [official docs][bsr-token]. Once you've
created a an API token, you need to create an encrypted [Github Secret][github-secret] for it. In
this example, the API token is set to the `BUF_TOKEN` secret.

## Prerequisites

For `buf-push-action` to run, you need to install the `buf` CLI in the GitHub Actions Runner first.
We recommend using [`buf-setup-action`][buf-setup] to install it (as in the example above).

## Configuration

| Parameter      | Description                                                                    | Required | Default                             |
|:---------------|:-------------------------------------------------------------------------------|:---------|:------------------------------------|
| `buf_token`    | The [Buf authentication token][buf-token] used for private [Buf inputs][input] | ✅        | [`${{github.token}}`][github-token] |
| `input`        | The path of the [input] you want to push to BSR as a module                    |          | `.`                                 |
| `track`        | The track to push to                                                           |          | `main`                              |
| `github_token` | The GitHub token to use when making API requests                               |          | [`${{github.token}}`][github-token] |

> These parameters are derived from [`action.yml`](./action.yml).

## Outputs
| Name         | Description                                     |
|--------------|-------------------------------------------------|
| `commit`     | The name of the commit that was pushed to BSR   |
| `commit_url` | A URL linking to the newly pushed commit on BSR |

## Common tasks

### Run against input in sub-directory

Some repositories are structured so that their [`buf.yaml`][buf-yaml] configuration file is defined
in a sub-directory alongside their Protobuf sources, such as a `proto` directory. Here's an example:

```sh
$ tree
.
└── proto
    ├── acme
    │   └── weather
    │       └── v1
    │           └── weather.proto
    └── buf.yaml
```

In that case, you can target the `proto` sub-directory by setting `input` to `proto`:

```yaml
steps:
  # Run `git checkout`
  - uses: actions/checkout@v2
  # Install the `buf` CLI
  - uses: bufbuild/buf-setup-action@v0.6.0
  # Push only the Input in `proto` to the BSR
  - uses: bufbuild/buf-push-action@v1
    with:
      input: proto
      buf_token: ${{ secrets.BUF_TOKEN }}
```

### Validate before push

`buf-push-action` is typically used alongside other `buf` Actions, such as
[`buf-breaking-action`][buf-breaking] and [`buf-lint-action`][buf-lint]. A common use case is to
"validate" a Buf module before pushing it to the [BSR] by ensuring that it passes both
[lint] and [breaking change][breaking] checks, as in this example:

```yaml
on: # Apply to all pushes to `main`
  push:
    branches:
      - main
jobs:
  validate-and-push-protos:
    runs-on: ubuntu-latest
    steps:
      # Run `git checkout`
      - uses: actions/checkout@v2
      # Install the `buf` CLI
      - uses: bufbuild/buf-setup-action@v0.6.0
      # Run a lint check on Protobuf sources
      - uses: bufbuild/buf-lint-action@v1
      # Run breaking change detection for Protobuf sources against the current `main` branch
      - uses: bufbuild/buf-breaking-action@v1
        with:
          against: https://github.com/acme/weather.git#branch=main,ref=HEAD~1,subdir=proto
      # Push the validated module to the BSR
      - uses: bufbuild/buf-push-action@v1
        with:
          buf_token: ${{ secrets.BUF_TOKEN }}
```

[breaking]: https://docs.buf.build/breaking
[bsr]: https://docs.buf.build/bsr
[bsr-token]: https://docs.buf.build/bsr/authentication
[buf-breaking]: https://github.com/marketplace/actions/buf-breaking
[buf-lint]: https://github.com/marketplace/actions/buf-lint
[buf-setup]: https://github.com/marketplace/actions/buf-setup
[buf-token]: https://docs.buf.build/bsr/authentication#create-an-api-token
[buf-yaml]: https://docs.buf.build/configuration/v1/buf-yaml
[github-secret]: https://docs.github.com/en/actions/reference/encrypted-secrets
[github-token]: https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
[input]: https://docs.buf.build/reference/inputs
[lint]: https://docs.buf.build/lint
[modules]: https://docs.buf.build/bsr/overview#module
