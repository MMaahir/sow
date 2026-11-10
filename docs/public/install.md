# Install

## With Homebrew

```sh
brew install MMaahir/tap/sow
```

## With Go

Replace `<version>` with a release tag, such as `v0.1.0`:

```sh
go install github.com/MMaahir/sow/cmd/sow@<version>
```

Go puts the binary in `$(go env GOPATH)/bin`. That directory must be on your `PATH`.

## Check the install

```sh
sow --version
```

The command prints the version you installed.
