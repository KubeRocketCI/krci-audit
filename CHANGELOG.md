<a name="unreleased"></a>
## [Unreleased]


<a name="v0.1.0"></a>
## v0.1.0 - 2026-07-06
### Features

- add release and codeql workflows
- add scheduled partition rotation and retention CronJob
- add optional namespace scoping to admission webhook
- add krci-audit read-only HTTP API
- add krci-audit admission capture and append-only store

### Bug Fixes

- suppress healthcheck logs by prioritizing Heartbeat middleware
- stop chart from creating unreliable DB credential Secrets
- align app image values path with CD Pipeline Operator convention

### Routine

- Align CI pipelines


[Unreleased]: https://github.com/KubeRocketCI/krci-audit/compare/v0.1.0...HEAD
