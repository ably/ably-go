## Contributing to Ably Go SDK

Because this package uses `internal` packages, all fork development has to happen under `$GOPATH/src/github.com/ably/ably-go` to prevent `use of internal package not allowed` errors.

1. Fork `github.com/ably/ably-go`
2. go to the `ably-go` directory: `cd $GOPATH/src/github.com/ably/ably-go`
3. add your fork as a remote: `git remote add fork git@github.com:your-username/ably-go`
4. create your feature branch: `git checkout -b my-new-feature`
5. commit your changes (`git commit -am 'Add some feature'`)
6. ensure you have added suitable tests and the test suite is passing for both JSON and MessagePack protocols (see the Running Tests section below for more details about tests including the commands to run them).
7. push to the branch: `git push fork my-new-feature`
8. create a new Pull Request.

### Previewing Godoc Locally

1. Install [godoc](https://pkg.go.dev/golang.org/x/tools/cmd/godoc) globally via `go get` and run at root

```bash
  godoc -http=:8000
```
- Open the link http://localhost:8000/ for viewing the documentation.

2. Export `godoc` using latest version of [gopages](https://pkg.go.dev/github.com/johnstarich/go/gopages#section-readme)
```bash
  gopages -brand-description "Go client library for Ably realtime messaging service." -brand-title "Ably Go SDK"
```
- `godoc html` is exported to `dist` and can be served using `python3 http.server`

```bash
  cd dist
  py -m http.server 8000
```
- Open the link http://localhost:8000/ for viewing the documentation.

### Running Tests

This project contains two types of test. Test which use the `ablytest` package and tests which dont. 

At each stage of the CI pipeline, test results are uploaded to a [test observability server](https://github.com/ably-labs/test-observability-server)

The tests which don't make use of the `ablytest` package are considered unit tests. These tests exist in files which are suffixed `_test.go`. They run in the CI pipeline at the step `Unit Tests`. They can be run locally with the command: 

```
go test -v -tags=unit ./...
``` 

When adding new unit tests, the following build tag must be included at the top of the file to exclude these tests from running in CI as part of the Integration test step. 

```
//go:build !integration
// +build !integration
```

#### Integration tests

The tests which use the package `ablytest` are considered integration tests. These tests take longer to run than the unit tests and are mostly run in a sandbox environment. They are dependent on the sandbox environment being available and will fail if the environment is experiencing issues. There are some known issues for random failures in a few of the tests, so some of these tests may fail unexpectedly from time to time.

Please note that these tests are not true integration tests as rather than using the public API, they rely on `export_test.go` to expose private functionality so that it can be tested. These tests exist in files which are suffixed `_integration_test.go`. They run twice in the CI pipeline at the steps `Integration Tests with JSON Protocol` and `Integration Tests with MessagePack Protocol`. To run these tests locally, they have a dependency on a git submodule being present, so it is necessary clone the project with:

```
git clone git@github.com:ably/ably-go.git --recurse-submodules
```
 
The tests can be run with the commands: 
 
```
export ABLY_PROTOCOL="application/json" && go test -tags=integration -p 1 -race -v -timeout 120m ./...
export ABLY_PROTOCOL="application/x-msgpack" && go test -tags=integration -p 1 -race -v -timeout 120m ./...
``` 

Depending on which protocol they are to be run for. It is also necessary to clean the test cache in between runs of these tests which can be done with the command: 

```
go clean -testcache
``` 

When adding new integration tests, the following build tag must be included at the top of the file to exclude these tests for running in CI as part of the Unit test step.

```
//go:build !unit
// +build !unit
```

### Release process

Starting with release 1.2, this library uses [semantic versioning](http://semver.org/). For each release, the following needs to be done:

1. Create a branch for the release, named like `release/1.2.3` (where `1.2.3` is the new version number)
2. Replace all references of the current version number with the new version number and commit the changes
3. Run [`github_changelog_generator`](https://github.com/github-changelog-generator/github-changelog-generator) to automate the update of the [CHANGELOG](./CHANGELOG.md). This may require some manual intervention, both in terms of how the command is run and how the change log file is modified. Your mileage may vary:
  - The command you will need to run will look something like this: `github_changelog_generator -u ably -p ably-go --since-tag v1.2.3 --output delta.md --token $GITHUB_TOKEN_WITH_REPO_ACCESS`. Generate token [here](https://github.com/settings/tokens/new?description=GitHub%20Changelog%20Generator%20token).
  - Using the command above, `--output delta.md` writes changes made after `--since-tag` to a new file
  - The contents of that new file (`delta.md`) then need to be manually inserted at the top of the `CHANGELOG.md`, changing the "Unreleased" heading and linking with the current version numbers
  - Also ensure that the "Full Changelog" link points to the new version tag instead of the `HEAD`
4. Commit this change: `git add CHANGELOG.md && git commit -m "Update change log."`
5. Make a PR against `main`
6. Once the PR is approved, merge it into `main`
7. Add a tag to the new `main` head commit and push to origin such as `git tag v1.2.3 && git push origin v1.2.3`
8. Create the release on Github, from the new tag, including populating the release notes
9. Create the entry on the [Ably Changelog](https://changelog.ably.com/) (via [headwayapp](https://headwayapp.co/))
