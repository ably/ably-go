# Change Log

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