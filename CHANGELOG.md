# Changelog

## [1.2.0-beta.0](https://github.com/mikael-mansson/drime-shell/compare/v1.1.0-beta.0...v1.2.0-beta.0) (2026-01-06)


### Features

* initial commit ([f022fa6](https://github.com/mikael-mansson/drime-shell/commit/f022fa6874472dd5f93c59d05e2748e243ad3e1c))
* **install:** add checksum verification to install scripts ([fd683c2](https://github.com/mikael-mansson/drime-shell/commit/fd683c2635764dd0d69e959b12a99636817d6430))
* **startup:** show immediate visual feedback on launch ([05bf4ad](https://github.com/mikael-mansson/drime-shell/commit/05bf4ad6d2236f2debf4133c392f163c87259c4c))


### Bug Fixes

* **ci:** pin golangci-lint version to v1.63.4 ([f962f09](https://github.com/mikael-mansson/drime-shell/commit/f962f09b9442292a3aae7ad45549c3d2068c6bec))
* **ci:** use golangci-lint v1.63.4 directly instead of broken v9 action ([08d74f8](https://github.com/mikael-mansson/drime-shell/commit/08d74f8cc29cc65caaa6cda43c1936b46176f3ac))
* **ci:** use latest golangci-lint and remove --fast flag ([ac4b5ea](https://github.com/mikael-mansson/drime-shell/commit/ac4b5ea36548e18423f24cb61577385c3cdecf5e))
* **commands:** use %w for error wrapping and env.Stderr for output ([7e76125](https://github.com/mikael-mansson/drime-shell/commit/7e76125123d97bd9478dedce1530097161a479cd))
* **docs:** correct markdown errors and GitHub username ([b01b76f](https://github.com/mikael-mansson/drime-shell/commit/b01b76ff44edf263212f7803adac7ada7b3bec55))
* handle errcheck lint error in download.go ([1b8ff44](https://github.com/mikael-mansson/drime-shell/commit/1b8ff449b0bc41cb65f0d2b84c930c7770c28873))
* handle missing Content-Length header in CheckResumeSupport ([94435d3](https://github.com/mikael-mansson/drime-shell/commit/94435d3d16d9bd984d58cea778a84708bbbaf18d))
* **install:** use correct checksums filename from release ([5106d44](https://github.com/mikael-mansson/drime-shell/commit/5106d447e93b0051c2547f90f1ff851aa7dc89d0))
* **lint:** add version: 2 to golangci-lint config ([10751cf](https://github.com/mikael-mansson/drime-shell/commit/10751cfa4a9e7983d95543bba595646e842430e0))
* release proofreading fixes ([75bb09d](https://github.com/mikael-mansson/drime-shell/commit/75bb09d6a08fca898833bfc072c5034afe1bae2c))
* remove skip-github-release to enable tag creation ([cb4a7ef](https://github.com/mikael-mansson/drime-shell/commit/cb4a7ef260730115f0b64eb0daa4a467aac0d10d))
* resolve pre-release issues ([596ee47](https://github.com/mikael-mansson/drime-shell/commit/596ee47e62cb0ad07ccccde363a7605faae72306))
* support legacy -N flag syntax in head/tail and fix piped stdin detection ([24353f5](https://github.com/mikael-mansson/drime-shell/commit/24353f500fdf0e3e4a32850ab0e026cbb74c58a1))

## [1.1.0-beta.0](https://github.com/mikael-mansson/drime-shell/compare/v1.0.0-beta.0...v1.1.0-beta.0) (2026-01-06)


### Features

* initial commit ([f022fa6](https://github.com/mikael-mansson/drime-shell/commit/f022fa6874472dd5f93c59d05e2748e243ad3e1c))
* **install:** add checksum verification to install scripts ([fd683c2](https://github.com/mikael-mansson/drime-shell/commit/fd683c2635764dd0d69e959b12a99636817d6430))
* **startup:** show immediate visual feedback on launch ([05bf4ad](https://github.com/mikael-mansson/drime-shell/commit/05bf4ad6d2236f2debf4133c392f163c87259c4c))


### Bug Fixes

* **ci:** pin golangci-lint version to v1.63.4 ([f962f09](https://github.com/mikael-mansson/drime-shell/commit/f962f09b9442292a3aae7ad45549c3d2068c6bec))
* **ci:** use golangci-lint v1.63.4 directly instead of broken v9 action ([08d74f8](https://github.com/mikael-mansson/drime-shell/commit/08d74f8cc29cc65caaa6cda43c1936b46176f3ac))
* **ci:** use latest golangci-lint and remove --fast flag ([ac4b5ea](https://github.com/mikael-mansson/drime-shell/commit/ac4b5ea36548e18423f24cb61577385c3cdecf5e))
* **commands:** use %w for error wrapping and env.Stderr for output ([7e76125](https://github.com/mikael-mansson/drime-shell/commit/7e76125123d97bd9478dedce1530097161a479cd))
* **docs:** correct markdown errors and GitHub username ([b01b76f](https://github.com/mikael-mansson/drime-shell/commit/b01b76ff44edf263212f7803adac7ada7b3bec55))
* handle errcheck lint error in download.go ([1b8ff44](https://github.com/mikael-mansson/drime-shell/commit/1b8ff449b0bc41cb65f0d2b84c930c7770c28873))
* handle missing Content-Length header in CheckResumeSupport ([94435d3](https://github.com/mikael-mansson/drime-shell/commit/94435d3d16d9bd984d58cea778a84708bbbaf18d))
* **lint:** add version: 2 to golangci-lint config ([10751cf](https://github.com/mikael-mansson/drime-shell/commit/10751cfa4a9e7983d95543bba595646e842430e0))
* release proofreading fixes ([75bb09d](https://github.com/mikael-mansson/drime-shell/commit/75bb09d6a08fca898833bfc072c5034afe1bae2c))
* resolve pre-release issues ([596ee47](https://github.com/mikael-mansson/drime-shell/commit/596ee47e62cb0ad07ccccde363a7605faae72306))
* support legacy -N flag syntax in head/tail and fix piped stdin detection ([24353f5](https://github.com/mikael-mansson/drime-shell/commit/24353f500fdf0e3e4a32850ab0e026cbb74c58a1))
