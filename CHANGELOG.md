<a name="unreleased"></a>
## [Unreleased]


<a name="v0.2.0"></a>
## v0.2.0 - 2026-09-16
### Features

- Update pgbackrest default image version
- add audit facets API for filter dropdown values
- add release pipeline with changelog generation
- add scheduled partition rotation and retention CronJob
- add optional namespace scoping to admission webhook
- add krci-audit read-only HTTP API
- add krci-audit admission capture and append-only store

### Bug Fixes

- suppress healthcheck logs by prioritizing Heartbeat middleware
- stop chart from creating unreliable DB credential Secrets
- align app image values path with CD Pipeline Operator convention

### Routine

- apply Dependabot version updates for Go modules and Actions
- bump vulnerable dependencies flagged by Dependabot
- configure Dependabot commit prefix to pass PR validation
- Update current development version
- Align CI pipelines


[Unreleased]: https://github.com/KubeRocketCI/krci-audit/compare/v0.2.0...HEAD
