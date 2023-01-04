# Change Log

## [1.2.11](https://github.com/ably/ably-go/tree/v1.2.11)

This release updates docstring API commentaries for public interfaces.

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.10...v1.2.11)

**Merged pull requests:**

- Publish godoc for every commit [\#561](https://github.com/ably/ably-go/pull/561) ([sacOO7](https://github.com/sacOO7))
- Update to canonical doc [\#556](https://github.com/ably/ably-go/pull/556) ([sacOO7](https://github.com/sacOO7))

## [1.2.10](https://github.com/ably/ably-go/tree/v1.2.10)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.9...v1.2.10)

**Implemented enhancements:**

- Implement `RSC15l4` \(retry requests on Cloudfront HTTP error\) [\#571](https://github.com/ably/ably-go/issues/571), in [\#572](https://github.com/ably/ably-go/pull/572) ([owenpearson](https://github.com/owenpearson), [sacOO7](https://github.com/sacOO7))

## [1.2.9](https://github.com/ably/ably-go/tree/v1.2.9)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.8...v1.2.9)

**Merged pull requests:**

- Test against go 1.19, and drop support for 1.17 [\#563](https://github.com/ably/ably-go/pull/563) ([amnonbc](https://github.com/amnonbc))
- fix GO-2022-0493 [\#562](https://github.com/ably/ably-go/pull/562) ([amnonbc](https://github.com/amnonbc))

## [1.2.8](https://github.com/ably/ably-go/tree/v1.2.8)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.7...v1.2.8) (2022-07-01)

**Closed issues:**

- Use the net/http/httptest package to write unit test\(s\) for websockets [\#541](https://github.com/ably/ably-go/issues/541)
- Migrate from deprecated websocket package [\#368](https://github.com/ably/ably-go/issues/368)

**Merged pull requests:**

- Add unit tests for websocket.go [\#549](https://github.com/ably/ably-go/pull/549) ([Rosalita](https://github.com/Rosalita))
- Update action [\#548](https://github.com/ably/ably-go/pull/548) ([Morganamilo](https://github.com/Morganamilo))
- Replace obsolete websocket library -- allows WebAssembly [\#433](https://github.com/ably/ably-go/pull/433) ([Jmgr](https://github.com/Jmgr))

## [1.2.7](https://github.com/ably/ably-go/tree/v1.2.7) (2022-06-09)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.6...v1.2.7)

**Implemented enhancements:**

- Add support to get channel lifecycle status  [\#509](https://github.com/ably/ably-go/issues/509)

**Fixed bugs:**

- Missing pointer safety when creating a new Realtime Client [\#537](https://github.com/ably/ably-go/issues/537)
- realtime: missing test for sync resume \(disconnection while sync is in progress\) [\#34](https://github.com/ably/ably-go/issues/34)

**Closed issues:**

- Remove dependency on test.sh bash script to run the tests [\#530](https://github.com/ably/ably-go/issues/530)
- Unstable test TestStats\_Direction\_RSC6b2 [\#520](https://github.com/ably/ably-go/issues/520)
- Remove flaky unit tests TestAfterOK and TestAfterCanceled in package ablyutil [\#514](https://github.com/ably/ably-go/issues/514)
- Tests for packages ablyutil and ablytest are running more than once in the CI pipeline [\#504](https://github.com/ably/ably-go/issues/504)
- Flaky POST /apps: "Unable to modify existing channel namespace" [\#354](https://github.com/ably/ably-go/issues/354)
- Document ably/generate.go [\#351](https://github.com/ably/ably-go/issues/351)
- Document the project structure in our contributing guide [\#350](https://github.com/ably/ably-go/issues/350)
- Lint code, probably using `gofmt` [\#346](https://github.com/ably/ably-go/issues/346)
- Conform ReadMe and create Contributing Document [\#340](https://github.com/ably/ably-go/issues/340)

**Merged pull requests:**

- fix nil pointer issue [\#543](https://github.com/ably/ably-go/pull/543) ([mohyour](https://github.com/mohyour))
- Use pointers to mock types which contain embedded sync.mutex [\#542](https://github.com/ably/ably-go/pull/542) ([Rosalita](https://github.com/Rosalita))
- Increased unit test coverage [\#534](https://github.com/ably/ably-go/pull/534) ([Rosalita](https://github.com/Rosalita))
- Chore/update readme [\#533](https://github.com/ably/ably-go/pull/533) ([mohyour](https://github.com/mohyour))
- Remove dependency on test.sh bash script to run the tests [\#531](https://github.com/ably/ably-go/pull/531) ([Rosalita](https://github.com/Rosalita))
- revert git commit 787e6e98e5dfc2bf848e2cfdbe64bcff901ded2a [\#528](https://github.com/ably/ably-go/pull/528) ([Rosalita](https://github.com/Rosalita))
- Fix small capitalisation error in function name [\#527](https://github.com/ably/ably-go/pull/527) ([Rosalita](https://github.com/Rosalita))
- Fix flakey test by using a subtest to separate test set-up from test execution [\#525](https://github.com/ably/ably-go/pull/525) ([Rosalita](https://github.com/Rosalita))
- Fixes test failure in scenario where test is retried as app already exists with namespace. [\#524](https://github.com/ably/ably-go/pull/524) ([Rosalita](https://github.com/Rosalita))
- log request time outs when attempting retry [\#523](https://github.com/ably/ably-go/pull/523) ([Rosalita](https://github.com/Rosalita))
- \#509 add support to get lifecycle status [\#521](https://github.com/ably/ably-go/pull/521) ([mohyour](https://github.com/mohyour))
- Remove unstable time based unit tests from package ablyutil [\#518](https://github.com/ably/ably-go/pull/518) ([Rosalita](https://github.com/Rosalita))
- Add code formatting check to CI pipeline and format all files [\#517](https://github.com/ably/ably-go/pull/517) ([Rosalita](https://github.com/Rosalita))
- update dependency stretchr/testify from 1.4.0 to 1.7.1 [\#516](https://github.com/ably/ably-go/pull/516) ([Rosalita](https://github.com/Rosalita))
- Split integration tests into separate workflow [\#515](https://github.com/ably/ably-go/pull/515) ([owenpearson](https://github.com/owenpearson))
- Increased internal unit test coverage for error.go [\#510](https://github.com/ably/ably-go/pull/510) ([Rosalita](https://github.com/Rosalita))
- \#342 - update codec dependency [\#506](https://github.com/ably/ably-go/pull/506) ([mohyour](https://github.com/mohyour))
- Tests/add tags [\#505](https://github.com/ably/ably-go/pull/505) ([Rosalita](https://github.com/Rosalita))

## [1.2.6](https://github.com/ably/ably-go/tree/v1.2.6) (2022-5-10)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.5...v1.2.6)

**Fixed bugs:**

- AblyREST.Request\(\) does not work [\#486](https://github.com/ably/ably-go/issues/486)
- Callback not invoked, despite message being received from server [\#434](https://github.com/ably/ably-go/issues/434)
- Undescriptive errors; possible broken fallback functionality [\#125](https://github.com/ably/ably-go/issues/125)
- HTTPPaginatedResult and PaginatedResult are missing some methods [\#120](https://github.com/ably/ably-go/issues/120)
- ID and Client ID is blank on realtime incoming messages. [\#58](https://github.com/ably/ably-go/issues/58)
- Do not persist authorise attributes force & timestamp [\#54](https://github.com/ably/ably-go/issues/54)

**Closed issues:**

- go Update urls in readme [\#491](https://github.com/ably/ably-go/issues/491)
- Research a new API Design that allows Ably to be mocked in unit tests. [\#488](https://github.com/ably/ably-go/issues/488)
- Once test observability is in place try to turn all tests back on. [\#479](https://github.com/ably/ably-go/issues/479)
- Support Go 1.18 [\#474](https://github.com/ably/ably-go/issues/474)
- Test improvements - Standardise test assertions [\#456](https://github.com/ably/ably-go/issues/456)
- Investigate re-enabling flaky tests. [\#438](https://github.com/ably/ably-go/issues/438)
- Skipped Test: TestPresenceGet\_RSP3\_RSP3a1 [\#415](https://github.com/ably/ably-go/issues/415)
- Skipped Test: TestPresenceHistory\_RSP4\_RSP4b3 [\#414](https://github.com/ably/ably-go/issues/414)
- Skipped Test: TestHistory\_Direction\_RSL2b2 [\#413](https://github.com/ably/ably-go/issues/413)
- Skipped Test: TestRealtimePresence\_EnsureChannelIsAttached [\#412](https://github.com/ably/ably-go/issues/412)
- Skipped Test: TestRealtimePresence\_Sync250 [\#411](https://github.com/ably/ably-go/issues/411)
- Skipped Test: TestRealtimeConn\_BreakConnLoopOnInactiveState [\#410](https://github.com/ably/ably-go/issues/410)
- Skipped Test: TestRealtimeConn\_RTN19b [\#407](https://github.com/ably/ably-go/issues/407)
- Skipped Test: TestRealtimeConn\_RTN23 [\#405](https://github.com/ably/ably-go/issues/405)
- Skipped Test: TestRealtimeConn\_RTN15d\_MessageRecovery [\#404](https://github.com/ably/ably-go/issues/404)
- Skipped Test: TestRealtimeConn\_RTN12\_Connection\_Close RTN12d [\#403](https://github.com/ably/ably-go/issues/403)
- Skipped Test: TestRealtime\_DontCrashOnCloseWhenEchoOff [\#400](https://github.com/ably/ably-go/issues/400)
- Skipped Test: TestRealtimeChannel\_RTL4\_Attach RTL4j2 [\#398](https://github.com/ably/ably-go/issues/398)
- Skipped Test: TestAuth\_RSA7c [\#393](https://github.com/ably/ably-go/issues/393)
- Document ably/export\_test.go [\#352](https://github.com/ably/ably-go/issues/352)
- Rename ClientOption type to something better describing its purpose [\#348](https://github.com/ably/ably-go/issues/348)
- Clean up ably/doc\_test.go Example\_paginatedResults\(\) [\#345](https://github.com/ably/ably-go/issues/345)
- Update golang.org/x/net dependency \(January 2019 \> June 2021\) [\#341](https://github.com/ably/ably-go/issues/341)
- Flaky test in 1.2 branch: TestRealtimePresence\_Sync250 [\#338](https://github.com/ably/ably-go/issues/338)
- Flaky test in 1.2 branch: TestRealtimeConn\_RTN19b [\#333](https://github.com/ably/ably-go/issues/333)
- ably-go: add logging to rest & realtime clients [\#38](https://github.com/ably/ably-go/issues/38)

**Merged pull requests:**

- Refactored code and tests for TM3 message encoding/decoding [\#501](https://github.com/ably/ably-go/pull/501) ([Morganamilo](https://github.com/Morganamilo))
- fix failing test [\#500](https://github.com/ably/ably-go/pull/500) ([mohyour](https://github.com/mohyour))
- Fix sync250 [\#499](https://github.com/ably/ably-go/pull/499) ([Morganamilo](https://github.com/Morganamilo))
- update documentation links from ably.io to ably.com [\#497](https://github.com/ably/ably-go/pull/497) ([Rosalita](https://github.com/Rosalita))
- Enable and fix some skipped tests [\#495](https://github.com/ably/ably-go/pull/495) ([Morganamilo](https://github.com/Morganamilo))
- Change secret name [\#494](https://github.com/ably/ably-go/pull/494) ([Morganamilo](https://github.com/Morganamilo))
- Turn flaky tests on  and fix failing tests [\#493](https://github.com/ably/ably-go/pull/493) ([mohyour](https://github.com/mohyour))
- Fix Request\(\) [\#492](https://github.com/ably/ably-go/pull/492) ([Morganamilo](https://github.com/Morganamilo))
- Test observe action [\#490](https://github.com/ably/ably-go/pull/490) ([Morganamilo](https://github.com/Morganamilo))
- Do not persist authorise attributes force & timestamp [\#489](https://github.com/ably/ably-go/pull/489) ([mohyour](https://github.com/mohyour))
- Add missing documentation comment for REST.Time\(\) [\#487](https://github.com/ably/ably-go/pull/487) ([Rosalita](https://github.com/Rosalita))
- Handle errors in Request\(\) [\#485](https://github.com/ably/ably-go/pull/485) ([Morganamilo](https://github.com/Morganamilo))
- Move examples to their own file [\#482](https://github.com/ably/ably-go/pull/482) ([Morganamilo](https://github.com/Morganamilo))
- Update empty message fields on receive  [\#481](https://github.com/ably/ably-go/pull/481) ([Morganamilo](https://github.com/Morganamilo))
- Update Go version to 1.17 in go.mod, add Go 1.18 to versions tested [\#480](https://github.com/ably/ably-go/pull/480) ([Rosalita](https://github.com/Rosalita))
- Add missing pagination methods [\#478](https://github.com/ably/ably-go/pull/478) ([mohyour](https://github.com/mohyour))
- Added some unit tests for realtime presence.  [\#477](https://github.com/ably/ably-go/pull/477) ([Rosalita](https://github.com/Rosalita))
- Standardise integration test assertions [\#476](https://github.com/ably/ably-go/pull/476) ([Rosalita](https://github.com/Rosalita))
- Remove named returns to increase code readability. [\#475](https://github.com/ably/ably-go/pull/475) ([Rosalita](https://github.com/Rosalita))
- Tests/standardise assertions pt4 [\#473](https://github.com/ably/ably-go/pull/473) ([Rosalita](https://github.com/Rosalita))
- Subscribe before attaching [\#471](https://github.com/ably/ably-go/pull/471) ([Morganamilo](https://github.com/Morganamilo))

## [1.2.5](https://github.com/ably/ably-go/tree/v1.2.5) (2022-03-09)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.4...v1.2.5)

**Fixed bugs:**

- Failure to get channel with slash [\#463](https://github.com/ably/ably-go/issues/463)
- Hang when joining presence [\#462](https://github.com/ably/ably-go/issues/462)
- Frequently, a client can not get their own presence from a channel after making one call to `channel.Presence.Enter`. [\#436](https://github.com/ably/ably-go/issues/436)

**Closed issues:**

- Test improvements - Move integration tests to their own package. [\#458](https://github.com/ably/ably-go/issues/458)
- SPIKE : Find a way to run a single integration test with a CLI command [\#449](https://github.com/ably/ably-go/issues/449)
- Test improvements - Separate unit tests and integration tests. [\#447](https://github.com/ably/ably-go/issues/447)
- Release ably-go [\#440](https://github.com/ably/ably-go/issues/440)

**Merged pull requests:**

- Expand docs on some functions [\#468](https://github.com/ably/ably-go/pull/468) ([Morganamilo](https://github.com/Morganamilo))
- Tests/standardise assertions pt2 [\#467](https://github.com/ably/ably-go/pull/467) ([Rosalita](https://github.com/Rosalita))
- Fix / not being escaped by rest client [\#466](https://github.com/ably/ably-go/pull/466) ([Morganamilo](https://github.com/Morganamilo))
- Part 1 of standardising test assertions [\#465](https://github.com/ably/ably-go/pull/465) ([Rosalita](https://github.com/Rosalita))
- Increased unit test coverage for auth.go [\#461](https://github.com/ably/ably-go/pull/461) ([Rosalita](https://github.com/Rosalita))
- Add detailed description of project tests to readme.md [\#460](https://github.com/ably/ably-go/pull/460) ([Rosalita](https://github.com/Rosalita))
- Split existing tests into unit and integration, add tags and add to CI [\#457](https://github.com/ably/ably-go/pull/457) ([Rosalita](https://github.com/Rosalita))
- Fix hang when joining existing channel with no presence [\#455](https://github.com/ably/ably-go/pull/455) ([Morganamilo](https://github.com/Morganamilo))
- Add presence message to presence map on enter [\#454](https://github.com/ably/ably-go/pull/454) ([Morganamilo](https://github.com/Morganamilo))
- Remove useless switch [\#453](https://github.com/ably/ably-go/pull/453) ([Morganamilo](https://github.com/Morganamilo))
- Add examples of publishing for both REST and realtime client. [\#444](https://github.com/ably/ably-go/pull/444) ([Rosalita](https://github.com/Rosalita))

## [1.2.4](https://github.com/ably/ably-go/tree/v1.2.4) (2022-02-10)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.3...v1.2.4)

**Fixed bugs:**

- Subscribe\(\), Attach\(\) are either counterintuitive or broken [\#155](https://github.com/ably/ably-go/issues/155)

**Closed issues:**

- Running tests locally, one test always fails. [\#445](https://github.com/ably/ably-go/issues/445)
- Add ConnectionKey to message [\#442](https://github.com/ably/ably-go/issues/442)
- Follow the lead of the Go team and support the last two major versions of Go [\#431](https://github.com/ably/ably-go/issues/431)
- Remove `go.mod` from `examples` [\#428](https://github.com/ably/ably-go/issues/428)
- Refine the documented release procedure [\#427](https://github.com/ably/ably-go/issues/427)
- Assess suitability of go 1.13 in our go.mod [\#344](https://github.com/ably/ably-go/issues/344)

**Merged pull requests:**

- Deprecate RESTChannel.PublishMultipleWithOptions [\#450](https://github.com/ably/ably-go/pull/450) ([lmars](https://github.com/lmars))
- Fix unit test that always failed when run locally. [\#446](https://github.com/ably/ably-go/pull/446) ([Rosalita](https://github.com/Rosalita))
- Add connection key to messages. [\#443](https://github.com/ably/ably-go/pull/443) ([Rosalita](https://github.com/Rosalita))
- Disable flaky tests. [\#439](https://github.com/ably/ably-go/pull/439) ([Rosalita](https://github.com/Rosalita))
- Add `ConnectionKey` to message [\#437](https://github.com/ably/ably-go/pull/437) ([ken8203](https://github.com/ken8203))
- Documentation improvements for RealtimePresence. [\#435](https://github.com/ably/ably-go/pull/435) ([Rosalita](https://github.com/Rosalita))
- examples: Remove go.mod and go.sum [\#430](https://github.com/ably/ably-go/pull/430) ([lmars](https://github.com/lmars))
- Update Go version to 1.16 in go.mod, drop \< 1.16 from versions tested, update README.md fixes \#431 [\#426](https://github.com/ably/ably-go/pull/426) ([Rosalita](https://github.com/Rosalita))

## [v1.2.3](https://github.com/ably/ably-go/tree/v1.2.3) (2021-10-27)

Sorry for the noise but we made a mistake when we released version 1.2.2 of this library. :facepalm:

The `v1.2.2` tag was pushed against the wrong commit, meaning that the library was announcing itself as at version 1.2.1 to the Ably service.
This release (assuming we get the tag push right this time :stuck_out_tongue_winking_eye:) will correctly identify itself as version 1.2.3.

## [v1.2.2](https://github.com/ably/ably-go/tree/v1.2.2) (2021-10-22)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.1...v1.2.2)

**Implemented enhancements:**

- Implement RSC7d \(Ably-Agent header\) [\#302](https://github.com/ably/ably-go/issues/302)

**Closed issues:**

- Ablytest [\#358](https://github.com/ably/ably-go/issues/358)
- RTN6: CONNECTED when connection is established [\#232](https://github.com/ably/ably-go/issues/232)
- RSA1, RSA2, RSA11: Basic auth [\#216](https://github.com/ably/ably-go/issues/216)

**Merged pull requests:**

- Ably Agent Refresh [\#424](https://github.com/ably/ably-go/pull/424) ([tomkirbygreen](https://github.com/tomkirbygreen))
- Fix/216 basic auth refresh [\#421](https://github.com/ably/ably-go/pull/421) ([tomkirbygreen](https://github.com/tomkirbygreen))
- Avoid a panic handling incoming ACK messages [\#418](https://github.com/ably/ably-go/pull/418) ([lmars](https://github.com/lmars))
- Fix incorrect usage of 'fmt.Sprintf' [\#392](https://github.com/ably/ably-go/pull/392) ([tomkirbygreen](https://github.com/tomkirbygreen))
- Fix various method contracts [\#387](https://github.com/ably/ably-go/pull/387) ([tomkirbygreen](https://github.com/tomkirbygreen))
- Update examples [\#376](https://github.com/ably/ably-go/pull/376) ([sacOO7](https://github.com/sacOO7))
- Clarify event emitter concurrency documentation [\#365](https://github.com/ably/ably-go/pull/365) ([tcard](https://github.com/tcard))
- Ably agent header [\#360](https://github.com/ably/ably-go/pull/360) ([sacOO7](https://github.com/sacOO7))

## [v1.2.1](https://github.com/ably/ably-go/tree/v1.2.1) (2021-08-09)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0...v1.2.1)

**Implemented enhancements:**

- Need a migration guide for users going from 1.1.5 to 1.2.0 [\#361](https://github.com/ably/ably-go/issues/361)
- Changelog and release tag [\#48](https://github.com/ably/ably-go/issues/48)

**Merged pull requests:**

- Update generated files [\#371](https://github.com/ably/ably-go/pull/371) ([lmars](https://github.com/lmars))
- Don't set message encoding for valid utf-8 string data [\#370](https://github.com/ably/ably-go/pull/370) ([lmars](https://github.com/lmars))
- Export the ablytest package [\#366](https://github.com/ably/ably-go/pull/366) ([tcard](https://github.com/tcard))
- Migration guide 1.1.5 to 1.2.0 [\#364](https://github.com/ably/ably-go/pull/364) ([QuintinWillison](https://github.com/QuintinWillison))
- Update README.md [\#362](https://github.com/ably/ably-go/pull/362) ([AndyNicks](https://github.com/AndyNicks))
- RTC8a: client-requested reauthorization while CONNECTED \(to `main` branch\) [\#357](https://github.com/ably/ably-go/pull/357) ([tcard](https://github.com/tcard))
- RTC8a: client-requested reauthorization while CONNECTED [\#336](https://github.com/ably/ably-go/pull/336) ([tcard](https://github.com/tcard))

## [v1.2.0](https://github.com/ably/ably-go/tree/v1.2.0) (2021-07-09)

Version 1.2.0 is out of pre-release!

[**Full Changelog since v1.1.5**](https://github.com/ably/ably-go/compare/v1.1.5...v1.2.0)

[Full Changelog since v1.2.0-apipreview.6](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.6...v1.2.0)

**Implemented enhancements:**

- Add enough logs to troubleshoot problems [\#160](https://github.com/ably/ably-go/issues/160)

**Closed issues:**

- Create code snippets for homepage \(go\) [\#324](https://github.com/ably/ably-go/issues/324)
- Unexport package proto, ablycrypto, ablytest [\#291](https://github.com/ably/ably-go/issues/291)
- API compliance - Go 1.2 [\#271](https://github.com/ably/ably-go/issues/271)
- RTN7: ACK and NACK [\#215](https://github.com/ably/ably-go/issues/215)

**Merged pull requests:**

- Fix conflicts from integration-1.2 to main [\#353](https://github.com/ably/ably-go/pull/353) ([sacOO7](https://github.com/sacOO7))
- Don't reuse IV between encryptions [\#331](https://github.com/ably/ably-go/pull/331) ([tcard](https://github.com/tcard))
- Integration/1.2 [\#322](https://github.com/ably/ably-go/pull/322) ([QuintinWillison](https://github.com/QuintinWillison))

## [v1.2.0-apipreview.6](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.6) (2021-06-17)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.5...v1.2.0-apipreview.6)

**Fixed bugs:**

- IV shouldn't be reused between messages [\#330](https://github.com/ably/ably-go/issues/330)

**Closed issues:**

- RTN21: Overriding connectionDetails [\#227](https://github.com/ably/ably-go/issues/227)

**Merged pull requests:**

- RTN7b: Handle ACK/NACK [\#335](https://github.com/ably/ably-go/pull/335) ([tcard](https://github.com/tcard))

## [v1.2.0-apipreview.5](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.5) (2021-06-04)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.4...v1.2.0-apipreview.5)

**Implemented enhancements:**

- Common design for PaginatedResult-related methods [\#278](https://github.com/ably/ably-go/issues/278)
- Add Connection Error Handling - 0.8 feature [\#51](https://github.com/ably/ably-go/issues/51)

**Fixed bugs:**

- API completeness [\#50](https://github.com/ably/ably-go/issues/50)
- Passing tests appear to have failed in Travis [\#47](https://github.com/ably/ably-go/issues/47)
- Go JSON / Binary / String support [\#9](https://github.com/ably/ably-go/issues/9)

**Closed issues:**

- Fix RTN14c test to include full connection establishment [\#315](https://github.com/ably/ably-go/issues/315)
- Flaky test in 1.2 branch: TestStatsPagination\_RSC6a\_RSCb3 [\#313](https://github.com/ably/ably-go/issues/313)
- Flaky test in 1.2 branch: TestPresenceHistory\_Direction\_RSP4b2 [\#310](https://github.com/ably/ably-go/issues/310)
- Flaky test in 1.2 branch: TestRealtimeChannel\_Detach [\#309](https://github.com/ably/ably-go/issues/309)
- Fix channel iteration methods  [\#307](https://github.com/ably/ably-go/issues/307)
- RTN12: Connection.close [\#262](https://github.com/ably/ably-go/issues/262)
- RTL2f: ChannelEvent RESUMED flag [\#243](https://github.com/ably/ably-go/issues/243)
- RTL17: Only dispatch messages when ATTACHED [\#240](https://github.com/ably/ably-go/issues/240)
- RTL14: ERROR message for channel [\#238](https://github.com/ably/ably-go/issues/238)
- RTL12: Random incoming ATTACHED message [\#237](https://github.com/ably/ably-go/issues/237)
- RTN3: Connection.autoConnect [\#229](https://github.com/ably/ably-go/issues/229)
- RTN2: WebSocket query params [\#226](https://github.com/ably/ably-go/issues/226)
- RTN10: Connection.serial [\#224](https://github.com/ably/ably-go/issues/224)
- RTN24: Handle random CONNECTED message [\#223](https://github.com/ably/ably-go/issues/223)
- RTL5: Channel.detach [\#212](https://github.com/ably/ably-go/issues/212)
- RTL4: Channel.attach [\#211](https://github.com/ably/ably-go/issues/211)
- Bring back reverted README examples [\#207](https://github.com/ably/ably-go/issues/207)

**Merged pull requests:**

- Unexport CipherParams.IV, useful only for tests [\#334](https://github.com/ably/ably-go/pull/334) ([tcard](https://github.com/tcard))
- Unexport package proto, ablycrypto, ablytest [\#332](https://github.com/ably/ably-go/pull/332) ([tcard](https://github.com/tcard))
- Fix/conflict integration 1.2 [\#329](https://github.com/ably/ably-go/pull/329) ([sacOO7](https://github.com/sacOO7))
- \[FIX CONFLICTS\] Merge env. fallbacks to 1.2 [\#328](https://github.com/ably/ably-go/pull/328) ([sacOO7](https://github.com/sacOO7))
- Replace all `ts \*testing.T` instances with `t` [\#326](https://github.com/ably/ably-go/pull/326) ([tcard](https://github.com/tcard))
- Simplify and uniformize logging [\#321](https://github.com/ably/ably-go/pull/321) ([tcard](https://github.com/tcard))
- Websocket query params [\#320](https://github.com/ably/ably-go/pull/320) ([sacOO7](https://github.com/sacOO7))
- Use persisted namespace for history tests [\#319](https://github.com/ably/ably-go/pull/319) ([tcard](https://github.com/tcard))
- Avoid current stats period interference with fixtures in tests [\#318](https://github.com/ably/ably-go/pull/318) ([tcard](https://github.com/tcard))
- Rewrite RTN14c to test full connection establishment [\#317](https://github.com/ably/ably-go/pull/317) ([tcard](https://github.com/tcard))
- Allow setting connection and request timeouts separately [\#312](https://github.com/ably/ably-go/pull/312) ([Jmgr](https://github.com/Jmgr))
- Remove flaky TestRealtimeChannel\_Detach [\#311](https://github.com/ably/ably-go/pull/311) ([tcard](https://github.com/tcard))
- Fix channel iteration [\#308](https://github.com/ably/ably-go/pull/308) ([sacOO7](https://github.com/sacOO7))
- Connection Autoconnect [\#306](https://github.com/ably/ably-go/pull/306) ([sacOO7](https://github.com/sacOO7))
- Connection serial [\#305](https://github.com/ably/ably-go/pull/305) ([sacOO7](https://github.com/sacOO7))
- Override connectionDetails [\#304](https://github.com/ably/ably-go/pull/304) ([sacOO7](https://github.com/sacOO7))
- Rename PublishBatch -\> PublishMultiple [\#303](https://github.com/ably/ably-go/pull/303) ([tcard](https://github.com/tcard))
- Migrate Presence.get and REST.request to new paginated results [\#301](https://github.com/ably/ably-go/pull/301) ([tcard](https://github.com/tcard))
- Remove Ping public Interface [\#300](https://github.com/ably/ably-go/pull/300) ([sacOO7](https://github.com/sacOO7))
- Channel Attach [\#299](https://github.com/ably/ably-go/pull/299) ([sacOO7](https://github.com/sacOO7))
- Channel Detach [\#298](https://github.com/ably/ably-go/pull/298) ([sacOO7](https://github.com/sacOO7))
- Connection Close [\#297](https://github.com/ably/ably-go/pull/297) ([sacOO7](https://github.com/sacOO7))
- Conform license and copyright [\#296](https://github.com/ably/ably-go/pull/296) ([QuintinWillison](https://github.com/QuintinWillison))
- RTN24, Handle Random Connected Message [\#295](https://github.com/ably/ably-go/pull/295) ([sacOO7](https://github.com/sacOO7))
- Channel message dispatch only when it's attached [\#294](https://github.com/ably/ably-go/pull/294) ([sacOO7](https://github.com/sacOO7))
- Adapt History to follow new paginated result design [\#292](https://github.com/ably/ably-go/pull/292) ([tcard](https://github.com/tcard))
- FailedChannelState on error message [\#289](https://github.com/ably/ably-go/pull/289) ([sacOO7](https://github.com/sacOO7))
- Channel resume [\#288](https://github.com/ably/ably-go/pull/288) ([sacOO7](https://github.com/sacOO7))
- Ensure generated code is up-to-date in CI [\#287](https://github.com/ably/ably-go/pull/287) ([tcard](https://github.com/tcard))
- Added H2 with Resources [\#285](https://github.com/ably/ably-go/pull/285) ([ramiro-nd](https://github.com/ramiro-nd))
- Amend workflow branch name [\#284](https://github.com/ably/ably-go/pull/284) ([owenpearson](https://github.com/owenpearson))
- Ably 1.2 examples  [\#283](https://github.com/ably/ably-go/pull/283) ([sacOO7](https://github.com/sacOO7))
- Adapt Stats to follow new paginated result design [\#281](https://github.com/ably/ably-go/pull/281) ([tcard](https://github.com/tcard))
- Add scripts/test.sh for running tests [\#279](https://github.com/ably/ably-go/pull/279) ([lmars](https://github.com/lmars))
- TestFixConnLeak\_ISSUE89: Change the way it detects closed conns [\#277](https://github.com/ably/ably-go/pull/277) ([tcard](https://github.com/tcard))
- Generate env fallbacks [\#268](https://github.com/ably/ably-go/pull/268) ([sacOO7](https://github.com/sacOO7))

## [v1.2.0-apipreview.4](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.4) (2021-02-11)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.3...v1.2.0-apipreview.4)

**Implemented enhancements:**

- Add missing context.Context arguments [\#275](https://github.com/ably/ably-go/issues/275)
- Defaults: Generate environment fallbacks [\#198](https://github.com/ably/ably-go/issues/198)
- Implement support for connection recovery and resume [\#52](https://github.com/ably/ably-go/issues/52)

**Closed issues:**

- 'Account disabled' error gets sent as DISCONNECTED [\#269](https://github.com/ably/ably-go/issues/269)
- ttl seems to be set to null in a created token request? [\#266](https://github.com/ably/ably-go/issues/266)
- v1.2 API design [\#197](https://github.com/ably/ably-go/issues/197)

**Merged pull requests:**

- Bump the version constants [\#280](https://github.com/ably/ably-go/pull/280) ([lmars](https://github.com/lmars))
- Propagate contexts everywhere [\#276](https://github.com/ably/ably-go/pull/276) ([tcard](https://github.com/tcard))
- Merge main into 1.2 [\#274](https://github.com/ably/ably-go/pull/274) ([tcard](https://github.com/tcard))
- Replace Travis with GitHub workflow [\#273](https://github.com/ably/ably-go/pull/273) ([QuintinWillison](https://github.com/QuintinWillison))
- Fix regression in TestAuth\_ClientID's flakiness. [\#272](https://github.com/ably/ably-go/pull/272) ([tcard](https://github.com/tcard))
- Minor changes for 1.2 API compliance [\#270](https://github.com/ably/ably-go/pull/270) ([tcard](https://github.com/tcard))
- Add maintainers file [\#267](https://github.com/ably/ably-go/pull/267) ([niksilver](https://github.com/niksilver))
- add RTN19 - Transport state side effect [\#170](https://github.com/ably/ably-go/pull/170) ([gernest](https://github.com/gernest))

## [v1.2.0-apipreview.3](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.3) (2020-11-17)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.2...v1.2.0-apipreview.3)

**Implemented enhancements:**

- RTL9: Channel presence [\#264](https://github.com/ably/ably-go/issues/264)

**Closed issues:**

- Missing lib=go-\<version\> querystring param in connections [\#209](https://github.com/ably/ably-go/issues/209)

**Merged pull requests:**

- Add examples for handling events and errors [\#265](https://github.com/ably/ably-go/pull/265) ([lmars](https://github.com/lmars))
- Add RTN2g [\#210](https://github.com/ably/ably-go/pull/210) ([gernest](https://github.com/gernest))

## [v1.2.0-apipreview.2](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.2) (2020-11-09)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.1...v1.2.0-apipreview.2)

**Merged pull requests:**

- RTL6c: Publish while not connected [\#208](https://github.com/ably/ably-go/pull/208) ([tcard](https://github.com/tcard))
- Update README for 1.2 preview, with associated fixes [\#206](https://github.com/ably/ably-go/pull/206) ([tcard](https://github.com/tcard))
- add rtn14 - Connection opening failures: [\#172](https://github.com/ably/ably-go/pull/172) ([gernest](https://github.com/gernest))

## [v1.2.0-apipreview.1](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.1) (2020-10-21)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.2.0-apipreview.0...v1.2.0-apipreview.1)

**Merged pull requests:**

- Implement options as package-level functions [\#205](https://github.com/ably/ably-go/pull/205) ([tcard](https://github.com/tcard))
- v1.2-compliant message and presence publish and subscribe [\#202](https://github.com/ably/ably-go/pull/202) ([tcard](https://github.com/tcard))

## [v1.2.0-apipreview.0](https://github.com/ably/ably-go/tree/v1.2.0-apipreview.0) (2020-10-16)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.1.5...v1.2.0-apipreview.0)

**Fixed bugs:**

- Rest.request call timing out doesn't result in the errorMessage being set in the httpPaginatedResponse? [\#192](https://github.com/ably/ably-go/issues/192)
- Channel attach fails in the DISCONNECTED state [\#189](https://github.com/ably/ably-go/issues/189)
- TestFixConnLeak\_ISSUE89 is flaky [\#132](https://github.com/ably/ably-go/issues/132)

**Closed issues:**

- Reauthentication with Presence [\#164](https://github.com/ably/ably-go/issues/164)

**Merged pull requests:**

- Remove things from public API that aren't in the spec [\#201](https://github.com/ably/ably-go/pull/201) ([tcard](https://github.com/tcard))
- Explicitly annotate enum vars so that godoc places them with the type [\#200](https://github.com/ably/ably-go/pull/200) ([tcard](https://github.com/tcard))
- Introduce ErrorCode type for predefined error codes [\#199](https://github.com/ably/ably-go/pull/199) ([tcard](https://github.com/tcard))
- Merge main into 1.2 [\#196](https://github.com/ably/ably-go/pull/196) ([tcard](https://github.com/tcard))
- Always include serial values in encoded protocol messages [\#195](https://github.com/ably/ably-go/pull/195) ([lmars](https://github.com/lmars))
- Handle non-PaginatedResult but otherwise valid HTTP error responses [\#194](https://github.com/ably/ably-go/pull/194) ([tcard](https://github.com/tcard))
- Add HTTPRequestTimeout option with 10s default [\#193](https://github.com/ably/ably-go/pull/193) ([tcard](https://github.com/tcard))
- Ad-hoc fix for enqueuing attach attempt when DISCONNECTED. [\#191](https://github.com/ably/ably-go/pull/191) ([tcard](https://github.com/tcard))
- RTL2: EventEmitter for channel events; remove old State [\#190](https://github.com/ably/ably-go/pull/190) ([tcard](https://github.com/tcard))
- RTN15e: Check that Connection.Key changes on reconnections. [\#188](https://github.com/ably/ably-go/pull/188) ([tcard](https://github.com/tcard))
- RTN15d: Add a test for message delivery on connection recovery. [\#187](https://github.com/ably/ably-go/pull/187) ([tcard](https://github.com/tcard))
- RTL13: Handle DETACHED while not DETACHING. [\#185](https://github.com/ably/ably-go/pull/185) ([tcard](https://github.com/tcard))
- Fix test for Heartbeat [\#183](https://github.com/ably/ably-go/pull/183) ([gernest](https://github.com/gernest))
- Fix race condition in RTN15i test [\#182](https://github.com/ably/ably-go/pull/182) ([tcard](https://github.com/tcard))
- RTN15g: Don't attempt resume after server has discarded state [\#181](https://github.com/ably/ably-go/pull/181) ([tcard](https://github.com/tcard))
- RTN15i: Add test for already existing functionality. [\#180](https://github.com/ably/ably-go/pull/180) ([tcard](https://github.com/tcard))
- RTN15h\*: Handle incoming DISCONNECTED while CONNECTED [\#179](https://github.com/ably/ably-go/pull/179) ([tcard](https://github.com/tcard))
- Temporarily skip test for RTN23. [\#178](https://github.com/ably/ably-go/pull/178) ([tcard](https://github.com/tcard))
- Fix data race in RTN23 test. [\#177](https://github.com/ably/ably-go/pull/177) ([tcard](https://github.com/tcard))
- Have a single Travis build as originally intended. [\#176](https://github.com/ably/ably-go/pull/176) ([tcard](https://github.com/tcard))
- proto: Decode ms durations as a time.Duration wrapper. [\#174](https://github.com/ably/ably-go/pull/174) ([tcard](https://github.com/tcard))
- Fix instances of old nil \*ChannelOptions becoming nil option func. [\#173](https://github.com/ably/ably-go/pull/173) ([tcard](https://github.com/tcard))
- remove unused fields from Connection [\#171](https://github.com/ably/ably-go/pull/171) ([gernest](https://github.com/gernest))
- Add RTN23- heartbeats [\#169](https://github.com/ably/ably-go/pull/169) ([gernest](https://github.com/gernest))
- Rename master to main [\#167](https://github.com/ably/ably-go/pull/167) ([QuintinWillison](https://github.com/QuintinWillison))
-  Add rtn16  [\#165](https://github.com/ably/ably-go/pull/165) ([gernest](https://github.com/gernest))
- v1.2 ChannelOptions [\#146](https://github.com/ably/ably-go/pull/146) ([tcard](https://github.com/tcard))
- v1.2 ClientOptions [\#145](https://github.com/ably/ably-go/pull/145) ([tcard](https://github.com/tcard))
- v1.2 event emitter for connection \(RTN4\) [\#144](https://github.com/ably/ably-go/pull/144) ([tcard](https://github.com/tcard))

## [v1.1.5](https://github.com/ably/ably-go/tree/v1.1.5) (2020-05-27)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.1.4...v1.1.5)

**Merged pull requests:**

- Add logs to help in troubleshooting RestClient http api calls [\#162](https://github.com/ably/ably-go/pull/162) ([gernest](https://github.com/gernest))
- Remove unused fields definition on RealtimeClient [\#161](https://github.com/ably/ably-go/pull/161) ([gernest](https://github.com/gernest))
- add implementation for  RTN15b-c spec [\#159](https://github.com/ably/ably-go/pull/159) ([gernest](https://github.com/gernest))

## [v1.1.4](https://github.com/ably/ably-go/tree/v1.1.4) (2020-04-24)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.1.3...v1.1.4)

**Fixed bugs:**

- Lib failing to retrying on 5xx if it can't parse the body [\#154](https://github.com/ably/ably-go/issues/154)
- Flaky TestAuth\_ClientID test [\#80](https://github.com/ably/ably-go/issues/80)
- Likely leaking goroutines in multiple places [\#68](https://github.com/ably/ably-go/issues/68)

**Closed issues:**

- Implementing reauthentication before or after token expires [\#153](https://github.com/ably/ably-go/issues/153)

**Merged pull requests:**

- Properly set  Error.StatusCode [\#157](https://github.com/ably/ably-go/pull/157) ([gernest](https://github.com/gernest))
- Use ClientOptions.TLSPort for tls connections [\#156](https://github.com/ably/ably-go/pull/156) ([gernest](https://github.com/gernest))
- RTN15a: Reconnect after networking error. [\#152](https://github.com/ably/ably-go/pull/152) ([tcard](https://github.com/tcard))
- Fix goroutine leaks [\#151](https://github.com/ably/ably-go/pull/151) ([tcard](https://github.com/tcard))
- Remove fmt.Println leftovers. [\#150](https://github.com/ably/ably-go/pull/150) ([tcard](https://github.com/tcard))
- Don't read from connection on inactive state. [\#149](https://github.com/ably/ably-go/pull/149) ([tcard](https://github.com/tcard))
- RTN23a: Receive from WebSocket with a timeout. [\#148](https://github.com/ably/ably-go/pull/148) ([tcard](https://github.com/tcard))

## [v1.1.3](https://github.com/ably/ably-go/tree/v1.1.3) (2020-04-01)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.1.2...v1.1.3)

**Fixed bugs:**

- Drop dependency on subpackage [\#134](https://github.com/ably/ably-go/issues/134)

**Closed issues:**

- Ambiguous import - found go-codec in multiple modules [\#135](https://github.com/ably/ably-go/issues/135)
- Remove develop branch [\#128](https://github.com/ably/ably-go/issues/128)

**Merged pull requests:**

- Fix flaky tests [\#139](https://github.com/ably/ably-go/pull/139) ([tcard](https://github.com/tcard))
- bug: TestRestClient Stats failure due to race cond with sandbox [\#138](https://github.com/ably/ably-go/pull/138) ([audiolion](https://github.com/audiolion))
- bug: unpinned range scope variables in tests [\#137](https://github.com/ably/ably-go/pull/137) ([audiolion](https://github.com/audiolion))
- Update github.com/ugorji/go/codec to v1.1.7 [\#136](https://github.com/ably/ably-go/pull/136) ([lmars](https://github.com/lmars))

## [v1.1.2](https://github.com/ably/ably-go/tree/v1.1.2)

[Full Changelog](https://github.com/ably/ably-go/compare/v1.1.1...v1.1.2)

**Fixed bugs:**

- Unhelpful error message when failing to parse an HTTP response [\#127](https://github.com/ably/ably-go/issues/127)

**Merged pull requests:**

- REST client tests: a few fixes for test regressions [\#130](https://github.com/ably/ably-go/pull/130) ([paddybyers](https://github.com/paddybyers))
- First fix for issue 127 [\#129](https://github.com/ably/ably-go/pull/129) ([paddybyers](https://github.com/paddybyers))
- Fixed some typos in comments [\#124](https://github.com/ably/ably-go/pull/124) ([Jmgr](https://github.com/Jmgr))

## [v1.1.1](https://github.com/ably/ably-go/tree/v1.1.1) (2019-02-06)
[Full Changelog](https://github.com/ably/ably-go/compare/v1.1.0...v1.1.1)

**Closed issues:**

- 'Readme' not up to date [\#112](https://github.com/ably/ably-go/issues/112)

**Merged pull requests:**

- fix  go.mod [\#119](https://github.com/ably/ably-go/pull/119) ([gernest](https://github.com/gernest))
- Use modules to remove vendored websocket library [\#117](https://github.com/ably/ably-go/pull/117) ([paddybyers](https://github.com/paddybyers))
- Specify msgpack decode option to return strings for appropriate types [\#116](https://github.com/ably/ably-go/pull/116) ([paddybyers](https://github.com/paddybyers))
- Proper Usage for RestChannels.Get Method in README.md [\#115](https://github.com/ably/ably-go/pull/115) ([lacriment](https://github.com/lacriment))
- Make IdempotentRestPublishing default to false for 1.1 [\#114](https://github.com/ably/ably-go/pull/114) ([paddybyers](https://github.com/paddybyers))
- Add 1.1 feature support matrix [\#111](https://github.com/ably/ably-go/pull/111) ([paddybyers](https://github.com/paddybyers))

## [v1.1.0](https://github.com/ably/ably-go/tree/v1.1.0) (2018-11-09)
[Full Changelog](https://github.com/ably/ably-go/compare/v0.8.2...v1.1.0)

**Merged pull requests:**

- remember successful fallback host [\#110](https://github.com/ably/ably-go/pull/110) ([gernest](https://github.com/gernest))
- Add idempotent publishing [\#105](https://github.com/ably/ably-go/pull/105) ([gernest](https://github.com/gernest))
- RSA7 [\#99](https://github.com/ably/ably-go/pull/99) ([gernest](https://github.com/gernest))

## [v0.8.2](https://github.com/ably/ably-go/tree/v0.8.2) (2018-11-01)
[Full Changelog](https://github.com/ably/ably-go/compare/v0.8.1...v0.8.2)

**Closed issues:**

- Revert broken encoding on 0.8.1 [\#106](https://github.com/ably/ably-go/issues/106)

**Merged pull requests:**

- release 0.8.2 [\#108](https://github.com/ably/ably-go/pull/108) ([gernest](https://github.com/gernest))
- don't set message data  encoding  to utf-8 when it is a string [\#107](https://github.com/ably/ably-go/pull/107) ([gernest](https://github.com/gernest))
- add Request\(\) API \(RSC19\) [\#104](https://github.com/ably/ably-go/pull/104) ([gernest](https://github.com/gernest))
- add href attribute to ErrorInfo [\#103](https://github.com/ably/ably-go/pull/103) ([gernest](https://github.com/gernest))
- add RSE2  GenerateRandomKey [\#102](https://github.com/ably/ably-go/pull/102) ([gernest](https://github.com/gernest))
- rsa10k cache server time offset [\#95](https://github.com/ably/ably-go/pull/95) ([gernest](https://github.com/gernest))
- Update message encoding and encrypt channel message [\#88](https://github.com/ably/ably-go/pull/88) ([gernest](https://github.com/gernest))

## [v0.8.1](https://github.com/ably/ably-go/tree/v0.8.1) (2018-10-12)
[Full Changelog](https://github.com/ably/ably-go/compare/v0.8.0-beta.1...v0.8.1)

**Fixed bugs:**

- Client appears to be leaking TCP connections/file descriptors  [\#89](https://github.com/ably/ably-go/issues/89)
- Library not sending X-Ably-Lib header \(RSC7b\) [\#69](https://github.com/ably/ably-go/issues/69)

**Merged pull requests:**

- Release v0.8.1 [\#101](https://github.com/ably/ably-go/pull/101) ([gernest](https://github.com/gernest))
- ensure  client's response body is closed [\#100](https://github.com/ably/ably-go/pull/100) ([gernest](https://github.com/gernest))
- remove ginkgo and improve test runtime [\#97](https://github.com/ably/ably-go/pull/97) ([gernest](https://github.com/gernest))
- add Auth.Authorize [\#96](https://github.com/ably/ably-go/pull/96) ([gernest](https://github.com/gernest))
- removed unused  code [\#94](https://github.com/ably/ably-go/pull/94) ([gernest](https://github.com/gernest))
- prefer the ably error codes generated by scripts/errors.go [\#93](https://github.com/ably/ably-go/pull/93) ([gernest](https://github.com/gernest))
- Fix conn leak [\#91](https://github.com/ably/ably-go/pull/91) ([gernest](https://github.com/gernest))
- fix Channel.Get API change in tests [\#86](https://github.com/ably/ably-go/pull/86) ([gernest](https://github.com/gernest))
- add ChannelOptions  [\#85](https://github.com/ably/ably-go/pull/85) ([gernest](https://github.com/gernest))
- add support for  rest channels [\#84](https://github.com/ably/ably-go/pull/84) ([gernest](https://github.com/gernest))
- RSA9h update Auth.CreateTokenRequest [\#83](https://github.com/ably/ably-go/pull/83) ([gernest](https://github.com/gernest))
-  add more tests for rsc15 [\#81](https://github.com/ably/ably-go/pull/81) ([gernest](https://github.com/gernest))
- Release 0.8.0 beta.1 [\#79](https://github.com/ably/ably-go/pull/79) ([ORBAT](https://github.com/ORBAT))
- add rsc15 support host fallback [\#78](https://github.com/ably/ably-go/pull/78) ([gernest](https://github.com/gernest))

## [v0.8.0-beta.1](https://github.com/ably/ably-go/tree/v0.8.0-beta.1) (2018-09-05)
[Full Changelog](https://github.com/ably/ably-go/compare/v0.8.0-beta.0...v0.8.0-beta.1)

**Implemented enhancements:**

- Contribution instructions out of date [\#62](https://github.com/ably/ably-go/issues/62)
- Update section about contributing [\#63](https://github.com/ably/ably-go/pull/63) ([ORBAT](https://github.com/ORBAT))

**Merged pull requests:**

- use constants for message encoding strings [\#77](https://github.com/ably/ably-go/pull/77) ([gernest](https://github.com/gernest))
- RSC7b add lib header [\#75](https://github.com/ably/ably-go/pull/75) ([gernest](https://github.com/gernest))
- call Close on sandbox when exiting TestRSC7 test [\#74](https://github.com/ably/ably-go/pull/74) ([gernest](https://github.com/gernest))
- Update stats object to 1.1 spec [\#73](https://github.com/ably/ably-go/pull/73) ([gernest](https://github.com/gernest))
-  Generate ably error status code [\#72](https://github.com/ably/ably-go/pull/72) ([gernest](https://github.com/gernest))
- RSC4 accept custom logger [\#71](https://github.com/ably/ably-go/pull/71) ([gernest](https://github.com/gernest))
- rsc7a add version header [\#70](https://github.com/ably/ably-go/pull/70) ([gernest](https://github.com/gernest))
- RSC7a add version header [\#67](https://github.com/ably/ably-go/pull/67) ([gernest](https://github.com/gernest))
- Update .travis.yml to a working state [\#66](https://github.com/ably/ably-go/pull/66) ([gernest](https://github.com/gernest))
- Use new  msgpack package [\#65](https://github.com/ably/ably-go/pull/65) ([gernest](https://github.com/gernest))
- Fix resource count  value types [\#61](https://github.com/ably/ably-go/pull/61) ([gernest](https://github.com/gernest))

## [v0.8.0-beta.0](https://github.com/ably/ably-go/tree/v0.8.0-beta.0) (2017-08-26)
**Implemented enhancements:**

- Switch arity of auth methods [\#44](https://github.com/ably/ably-go/issues/44)
- Spec validation [\#31](https://github.com/ably/ably-go/issues/31)
- add ably.Error error type  [\#30](https://github.com/ably/ably-go/issues/30)
- proto: improve Stats struct [\#29](https://github.com/ably/ably-go/issues/29)
- rest: add support for token auth for requests [\#28](https://github.com/ably/ably-go/issues/28)
- Consistent README [\#21](https://github.com/ably/ably-go/issues/21)
- API changes Apr 2015 [\#19](https://github.com/ably/ably-go/issues/19)
- API changes [\#15](https://github.com/ably/ably-go/issues/15)
- Update imports to reference vendored packages [\#3](https://github.com/ably/ably-go/pull/3) ([lmars](https://github.com/lmars))

**Fixed bugs:**

- Tests for Go 1.4.3 hang [\#46](https://github.com/ably/ably-go/issues/46)
- Switch arity of auth methods [\#44](https://github.com/ably/ably-go/issues/44)
- rest: add tests for URL-escaping channel names [\#42](https://github.com/ably/ably-go/issues/42)
- realtime: Presence.Get\(\) should attach instead of raising error [\#36](https://github.com/ably/ably-go/issues/36)
- rest: check \(\*http.Response\).StatusCode codes for != 200 [\#27](https://github.com/ably/ably-go/issues/27)
- rest: make ProtocolMsgPack default [\#26](https://github.com/ably/ably-go/issues/26)
- rest: set proper request headers for msgpack protocol [\#25](https://github.com/ably/ably-go/issues/25)
- rest: proto.Stat serialization failure [\#23](https://github.com/ably/ably-go/issues/23)
- Fix REST/Presence tests to not fail randomly. [\#20](https://github.com/ably/ably-go/issues/20)
- API changes Apr 2015 [\#19](https://github.com/ably/ably-go/issues/19)
- API changes [\#15](https://github.com/ably/ably-go/issues/15)

**Closed issues:**

- Use ably-common for test data [\#14](https://github.com/ably/ably-go/issues/14)
- Support client instantiation via Token or TokenRequest [\#12](https://github.com/ably/ably-go/issues/12)

**Merged pull requests:**

- A few fixes to README. [\#55](https://github.com/ably/ably-go/pull/55) ([tcard](https://github.com/tcard))
- auth: latest spec additions [\#45](https://github.com/ably/ably-go/pull/45) ([rjeczalik](https://github.com/rjeczalik))
- realtime: increase test coverage for Connection / Channel [\#43](https://github.com/ably/ably-go/pull/43) ([rjeczalik](https://github.com/rjeczalik))
- auth: add token authentication  [\#41](https://github.com/ably/ably-go/pull/41) ([rjeczalik](https://github.com/rjeczalik))
- Add Makefile [\#40](https://github.com/ably/ably-go/pull/40) ([lmars](https://github.com/lmars))
- MsgPack support; reworked Stat data type; updated deps [\#39](https://github.com/ably/ably-go/pull/39) ([rjeczalik](https://github.com/rjeczalik))
- realtime: attach to channel on Presence.Get [\#37](https://github.com/ably/ably-go/pull/37) ([rjeczalik](https://github.com/rjeczalik))
- fix RealtimePresence race; add StateEnum type [\#35](https://github.com/ably/ably-go/pull/35) ([rjeczalik](https://github.com/rjeczalik))
- ably: add RealtimePresence [\#33](https://github.com/ably/ably-go/pull/33) ([rjeczalik](https://github.com/rjeczalik))
- realtime client - first working version [\#32](https://github.com/ably/ably-go/pull/32) ([rjeczalik](https://github.com/rjeczalik))
- rest: improve RestClient API [\#24](https://github.com/ably/ably-go/pull/24) ([rjeczalik](https://github.com/rjeczalik))
- Auth API changes [\#22](https://github.com/ably/ably-go/pull/22) ([rjeczalik](https://github.com/rjeczalik))
- Refactor proto to use \[\]byte for binary data. [\#18](https://github.com/ably/ably-go/pull/18) ([rjeczalik](https://github.com/rjeczalik))
- Refactor Params -\> ClientOptions and logging [\#17](https://github.com/ably/ably-go/pull/17) ([rjeczalik](https://github.com/rjeczalik))
- Test ably/proto with ably-common fixtures [\#16](https://github.com/ably/ably-go/pull/16) ([rjeczalik](https://github.com/rjeczalik))
- Merge config, rest and realtime into one ably package [\#13](https://github.com/ably/ably-go/pull/13) ([rjeczalik](https://github.com/rjeczalik))
- Add \(\*PaginatedResource\).Items\(\) for accessing interface{} items [\#11](https://github.com/ably/ably-go/pull/11) ([rjeczalik](https://github.com/rjeczalik))
- Rework proto.PaginatedResource API [\#10](https://github.com/ably/ably-go/pull/10) ([rjeczalik](https://github.com/rjeczalik))
- Race fixes plus updated Travis CI configuration [\#8](https://github.com/ably/ably-go/pull/8) ([rjeczalik](https://github.com/rjeczalik))
- Don't verify hostname if HTTP\_PROXY is set [\#7](https://github.com/ably/ably-go/pull/7) ([rjeczalik](https://github.com/rjeczalik))
- Fix capability [\#6](https://github.com/ably/ably-go/pull/6) ([rjeczalik](https://github.com/rjeczalik))
- Add support to encode data in messages [\#5](https://github.com/ably/ably-go/pull/5) ([kouno](https://github.com/kouno))
- Rest presence [\#4](https://github.com/ably/ably-go/pull/4) ([kouno](https://github.com/kouno))
- Structure change [\#2](https://github.com/ably/ably-go/pull/2) ([kouno](https://github.com/kouno))
- Move to Ginkgo and add Godep for dependencies management [\#1](https://github.com/ably/ably-go/pull/1) ([kouno](https://github.com/kouno))



\* *This Change Log was automatically generated by [github_changelog_generator](https://github.com/skywinder/Github-Changelog-Generator)*
