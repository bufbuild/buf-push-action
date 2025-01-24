# `buf-push-action`

> [!NOTE]
> This action has been deprecated in favor of the [`buf-action`][buf-action] which combines the
> functionality of `buf-push-action` with the ability to run Buf commands in the same step. Please
> see the [migration guide][buf-action-migration] for more information.

This Action enables you to push [Buf modules][modules] to the [Buf Schema Registry][bsr] (BSR)
Pushed modules are created with the Git commit SHA as the module tag.

`buf-push-action` is frequently used alongside other Buf Actions, such as
[`buf-breaking-action`][buf-breaking] and [`buf-lint-action`][buf-lint].

## Usage

Here's an example usage of `buf-push-action`:

```yaml
on: push # Apply to all push
jobs:
  push-module:
    runs-on: ubuntu-latest
    steps:
      # Run `git checkout`
      - uses: actions/checkout@v2
      # Install the `buf` CLI
      - uses: bufbuild/buf-setup-action@v1
      # Push module to the BSR
      - uses: bufbuild/buf-push-action@v1
        with:
          buf_token: ${{ secrets.BUF_TOKEN }}
          create_visibility: private
          draft: ${{ github.ref_name != 'main'}}
```

With this configuration, upon a push [branches, tags][github-workflow]
the `buf` CLI pushes the [configured module][buf-yaml] to the BSR using the provided to
authenticate the request. If the repository does not already exist on the BSR, create it
with private visibility. When the triggering branch is not `main`, the commit will be pushed
as a [draft][buf-draft].

For instructions on creating a BSR API token, see our [official docs][bsr-token]. Once you've
created a an API token, you need to create an encrypted [Github Secret][github-secret] for it. In
this example, the API token is set to the `BUF_TOKEN` secret.

## Prerequisites

For `buf-push-action` to run, you need to install the `buf` CLI in the GitHub Actions Runner first.
We recommend using [`buf-setup-action`][buf-setup] to install it (as in the example above).

## Configuration

| Parameter           | Description                                                                    | Required | Default                             |
| :------------------ | :----------------------------------------------------------------------------- | :------- | :---------------------------------- |
| `buf_token`         | The [Buf authentication token][buf-token] used for private [Buf inputs][input] | ✅       | [`${{github.token}}`][github-token] |
| `input`             | The path of the [input] you want to push to BSR as a module                    |          | `.`                                 |
| `draft`             | Indicates if the workflows should push to the BSR as a [draft][buf-draft]      |          |                                     |
| `create_visibility` | The visibility to create the BSR repository with, if it does not already exist |          |                                     |

> These parameters are derived from [`action.yml`](./action.yml).

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
  - uses: bufbuild/buf-setup-action@v1
  # Push only the Input in `proto` to the BSR
  - uses: bufbuild/buf-push-action@v1
    with:
      input: proto
      buf_token: ${{ secrets.BUF_TOKEN }}
```

### Push multiple modules

If you have multiple modules defined in your repository, you'll need to configure `buf-push-action`
for each of the modules you want to push.

For example, suppose you have the following `buf.work.yaml`:

```yaml
version: v1
directories:
  - petapis
  - paymentapis
```

If you want to push both of the modules defined in the `paymentapis` and `petapis` directories,
you could adapt the workflow above like so (replacing the `proto` directory input with the
hypothetical `paymentapis` and `petapis` directory inputs):

```yaml
steps:
  # Run `git checkout`
  - uses: actions/checkout@v2
  # Install the `buf` CLI
  - uses: bufbuild/buf-setup-action@v1
  # Push only the Input in `paymentapis` to the BSR
  - uses: bufbuild/buf-push-action@v1
    with:
      input: paymentapis
      buf_token: ${{ secrets.BUF_TOKEN }}
  # Push only the Input in `petapis` to the BSR
  - uses: bufbuild/buf-push-action@v1
    with:
      input: petapis
      buf_token: ${{ secrets.BUF_TOKEN }}
```

Note, if any of the modules defined in your workspace depend on each other, you usually need to
run `buf mod update` so that the downstream module uses the upstream module's latest commit. This
is not supported by `buf-push-action` on its own - you'll need to stitch this functionality into
your workflow on your own. For more details, see [this](https://github.com/bufbuild/buf/issues/838)
discussion.

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
      - uses: bufbuild/buf-setup-action@v1
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

[buf-action]: https://github.com/bufbuild/buf-action
[buf-action-migration]: https://github.com/bufbuild/buf-action/blob/main/MIGRATION.md#buf-push-action
[breaking]: https://docs.buf.build/breaking
[bsr]: https://docs.buf.build/bsr
[bsr-token]: https://docs.buf.build/bsr/authentication
[buf-breaking]: https://github.com/marketplace/actions/buf-breaking
[buf-draft]: https://docs.buf.build/bsr/overview#referencing-a-module
[buf-lint]: https://github.com/marketplace/actions/buf-lint
[buf-setup]: https://github.com/marketplace/actions/buf-setup
[buf-token]: https://docs.buf.build/bsr/authentication#create-an-api-token
[buf-yaml]: https://docs.buf.build/configuration/v1/buf-yaml
[github-secret]: https://docs.github.com/en/actions/reference/encrypted-secrets
[github-token]: https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
[github-workflow]: https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#push
[input]: https://docs.buf.build/reference/inputs
[lint]: https://docs.buf.build/lint
[modules]: https://docs.buf.build/bsr/overview#module
