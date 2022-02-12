# `buf-push-action`

This Action enables you to push [Buf modules][modules] to the [Buf Schema Registry][bsr] (BSR)
Pushed modules are created with the Git commit SHA as the module tag.

`buf-push-action` is frequently used alongside other Buf Actions, such as
[`buf-breaking-action`][buf-breaking] and [`buf-lint-action`][buf-lint].

## Usage

Here's an example usage of `buf-push-action`:

```yaml
on: pull_request # Apply to all pull requests
jobs:
  push-module:
    # Run `git checkout`
    - uses: actions/checkout@v2
    # Install the `buf` CLI
    - uses: bufbuild/buf-setup-action@v0.6.0
    # Push module to the BSR
    - uses: bufbuild/buf-push-action@v1
      with:
        buf_token: ${{ secrets.BUF_TOKEN }}
```

With this configuration, the `buf` CLI pushes the [configured module][buf-yaml] to the BSR upon
merge.

## Prerequisites

For `buf-push-action` to run, you need to install the `buf` CLI in the GitHub Actions Runner first.
We recommend using [`buf-setup-action`][buf-setup] to install it (as in the example above).

## Configuration

Parameter | Description | Required | Default
:---------|:------------|:---------|:-------
`buf_token` | The [Buf authentication token][buf-token] used for private [Inputs][input] | ✅  | [`${{github.token}}`][github-token]
`input` | The path of the [Input] you want to push to BSR as a module | | `.`

> These parameters are derived from [`action.yml`][./action.yml].

## Common tasks



---

## Usage

Refer to the [action.yml](https://github.com/bufbuild/buf-push-action/blob/master/action.yml)
to see all of the action parameters.

The `buf-push` action requires that `buf` is installed in the Github Action
runner, so we'll use the [buf-setup][1] action to install it.

### Basic

In most cases, all you'll need to do is configure [buf-setup][1] and the
`buf_token` (used to authenticate access to the BSR). For details on
creating a `buf` API token, please refer to the
[documentation](https://beta.docs.buf.build/authentication#create-an-api-token).

Once you've created a `buf` API token, you'll need to create an encrypted
[Github Secret](https://docs.github.com/en/actions/reference/encrypted-secrets)
for it. In the following example, the API token is set to `BUF_TOKEN`.

```yaml
steps:
  - uses: actions/checkout@v2
  - uses: bufbuild/buf-setup-action@v0.6.0
  - uses: bufbuild/buf-push-action@v1
    with:
      buf_token: ${{ secrets.BUF_TOKEN }}
```

### Inputs

Some repositories are structured so that their `buf.yaml` is defined
in a sub-directory alongside their Protobuf sources, such as a `proto/`
directory. In this case, you can specify the relative `input` path.

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

```yaml
steps:
  - uses: actions/checkout@v2
  - uses: bufbuild/buf-setup-action@v0.6.0
  - uses: bufbuild/buf-push-action@v1
    with:
      input: 'proto'
      buf_token: ${{ secrets.BUF_TOKEN }}
```

### Validate before push

The `buf-push` action is also commonly used alongside other `buf` actions,
such as [buf-breaking][2] and [buf-lint][3].

In combination, you can verify that your module passes both `buf-lint`
and `buf-breaking` before the module is pushed to the BSR. The following example
uses the hypothetical `https://github.com/acme/weather.git` repository.

```yaml
on:
  push:
    branches:
      - main
jobs:
  validate-and-push-protos:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: bufbuild/buf-setup-action@v0.6.0
      - uses: bufbuild/buf-lint-action@v1
        with:
          input: 'proto'
      - uses: bufbuild/buf-breaking-action@v1
        with:
          input: 'proto'
          against: 'https://github.com/acme/weather.git#branch=master,ref=HEAD~1,subdir=proto'
      - uses: bufbuild/buf-push-action@v1
        with:
          input: 'proto'
          buf_token: ${{ secrets.BUF_TOKEN }}
```

[bsr]: https://docs.buf.build/bsr
[buf-breaking]: https://github.com/marketplace/actions/buf-breaking
[buf-lint]: https://github.com/marketplace/actions/buf-lint
[buf-setup]: https://github.com/marketplace/actions/buf-setup
[buf-token]: https://docs.buf.build/bsr/authentication#create-an-api-token
[buf-yaml]: https://docs.buf.build/configuration/v1/buf-yaml
[github-token]: https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
[input]: https://docs.buf.build/reference/inputs
[modules]: https://docs.buf.build/bsr/overview#module
