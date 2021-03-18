# buf-push-action

Push [buf](https://github.com/bufbuild/buf) modules to the
[Buf Schema Registry](https://buf.build) (BSR). The pushed
module will be created with a module tag equal to the git
commit SHA.

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
  - uses: bufbuild/buf-setup-action@v0.1.0
    with:
      version: '0.40.0'
  - uses: bufbuild/buf-push-action@v0.1.0
    with:
      github_token: ${{ github.token }}
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
  - uses: bufbuild/buf-setup-action@v0.1.0
    with:
      version: '0.40.0'
  - uses: bufbuild/buf-push-action@v0.1.0
    with:
      input: 'proto'
      github_token: ${{ github.token }}
      buf_token: ${{ secrets.BUF_TOKEN }}
```

### Validate before push

The `buf-push` action is also commonly used alongside other `buf` actions,
such as [buf-breaking][2] and [buf-lint][3].

In combination, you can verify that your module passes both `buf-breaking`
and `buf-lint` before the module is pushed to the BSR. The following example
uses the hypothetical `https://github.com/acme/weather.git` repository,
and includes a few additional `if` conditions to do exactly this.

```yaml
on:
  push:
    branches:
      - main
steps:
  - uses: actions/checkout@v2
  - uses: bufbuild/buf-setup-action@v0.1.0
    id: setup
    with:
      version: '0.40.0'
  - uses: bufbuild/buf-breaking-action@v0.1.0
    if: ${{ steps.setup.outcome == 'success' }}
    env:
      BUF_INPUT_HTTPS_USERNAME: ${{ github.actor }}
      BUF_INPUT_HTTPS_PASSWORD: ${{ github.token }}
    with:
      input: 'proto'
      against: 'https://github.com/acme/weather.git#branch=master,ref=HEAD~1,subdir=proto'
      github_token: ${{ github.token }}
  - uses: bufbuild/buf-lint-action@v0.1.0
    if: ${{ steps.setup.outcome == 'success' }}
    with:
      input: 'proto'
      github_token: ${{ github.token }}
  - uses: bufbuild/buf-push-action@v0.1.0
    if: success() # Only trigger the 'buf-push-action' if all previous steps succeed.
    with:
      input: 'proto'
      buf_token: ${{ secrets.BUF_TOKEN }}
```

  [1]: https://github.com/marketplace/actions/buf-setup
  [2]: https://github.com/marketplace/actions/buf-breaking
  [3]: https://github.com/marketplace/actions/buf-lint
