# krci-audit

![Version: 0.1.0-SNAPSHOT](https://img.shields.io/badge/Version-0.1.0--SNAPSHOT-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0-SNAPSHOT](https://img.shields.io/badge/AppVersion-0.1.0--SNAPSHOT-informational?style=flat-square)

A Helm chart for krci-audit — platform-agnostic Kubernetes admission audit capture & store

**Homepage:** <https://docs.kuberocketci.io/>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| epmd-edp | <SupportEPMD-EDP@epam.com> | <https://solutionshub.epam.com/solution/kuberocketci> |

## Source Code

* <https://github.com/KubeRocketCI/krci-audit>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| api.enabled | bool | `true` |  |
| api.port | int | `8080` |  |
| api.replicaCount | int | `1` |  |
| api.resources.limits.cpu | string | `"500m"` |  |
| api.resources.limits.memory | string | `"128Mi"` |  |
| api.resources.requests.cpu | string | `"10m"` |  |
| api.resources.requests.memory | string | `"32Mi"` |  |
| capture.failurePolicy | string | `"Ignore"` |  |
| capture.filter.groupResources[0].group | string | `"tekton.dev"` |  |
| capture.filter.groupResources[0].resource | string | `"pipelineruns"` |  |
| capture.filter.groups[0] | string | `"v2.edp.epam.com"` |  |
| capture.includeSubresources | list | `[]` |  |
| capture.level | string | `"metadata"` |  |
| capture.namespaces | list | `[]` |  |
| capture.operations[0] | string | `"CREATE"` |  |
| capture.operations[1] | string | `"UPDATE"` |  |
| capture.operations[2] | string | `"DELETE"` |  |
| capture.rules[0].apiGroups[0] | string | `"v2.edp.epam.com"` |  |
| capture.rules[0].apiVersions[0] | string | `"*"` |  |
| capture.rules[0].resources[0] | string | `"*"` |  |
| capture.rules[0].scope | string | `"*"` |  |
| capture.rules[1].apiGroups[0] | string | `"tekton.dev"` |  |
| capture.rules[1].apiVersions[0] | string | `"*"` |  |
| capture.rules[1].resources[0] | string | `"pipelineruns"` |  |
| capture.rules[1].scope | string | `"Namespaced"` |  |
| capture.storeRaw | bool | `false` |  |
| capture.timeoutSeconds | int | `1` |  |
| db.host | string | `""` |  |
| db.mode | string | `"external"` |  |
| db.name | string | `"krci-audit"` |  |
| db.owner.password | string | `""` |  |
| db.owner.passwordKey | string | `"password"` |  |
| db.owner.secretName | string | `""` |  |
| db.owner.userKey | string | `"user"` |  |
| db.pgo.backups.enabled | bool | `true` |  |
| db.pgo.backups.image | string | `"registry.developers.crunchydata.com/crunchydata/crunchy-pgbackrest:ubi8-2.51-0"` |  |
| db.pgo.backups.storage | string | `"2Gi"` |  |
| db.pgo.image | string | `"registry.developers.crunchydata.com/crunchydata/crunchy-postgres:ubi8-16.4-0"` |  |
| db.pgo.postgresVersion | int | `16` |  |
| db.pgo.replicas | int | `1` |  |
| db.pgo.storage | string | `"2Gi"` |  |
| db.port | int | `5432` |  |
| db.reader.password | string | `""` |  |
| db.reader.passwordKey | string | `"password"` |  |
| db.reader.secretName | string | `""` |  |
| db.simple.image | string | `"postgres:16-alpine"` |  |
| db.simple.persistence | bool | `true` |  |
| db.simple.resources.limits.cpu | string | `"1"` |  |
| db.simple.resources.limits.memory | string | `"512Mi"` |  |
| db.simple.resources.requests.cpu | string | `"50m"` |  |
| db.simple.resources.requests.memory | string | `"128Mi"` |  |
| db.simple.storage | string | `"2Gi"` |  |
| db.sslmode | string | `"disable"` |  |
| db.writer.password | string | `""` |  |
| db.writer.passwordKey | string | `"password"` |  |
| db.writer.secretName | string | `""` |  |
| fullnameOverride | string | `"krci-audit"` |  |
| imagePullSecrets | list | `[]` |  |
| images.app.pullPolicy | string | `"IfNotPresent"` |  |
| images.app.repository | string | `"epamedp/krci-audit"` |  |
| images.app.tag | string | `""` |  |
| images.kubeAuditRest.pullPolicy | string | `"IfNotPresent"` |  |
| images.kubeAuditRest.repository | string | `"ghcr.io/richardoc/kube-audit-rest"` |  |
| images.kubeAuditRest.tag | string | `"ad68f71978e8cd610b5b06769fab301cf9ee74d0-distroless@sha256:2444c1207156681c4ed04e7bb02662820c9bfb31b50e8fe5b0112b3f8f577d42"` |  |
| images.vector.pullPolicy | string | `"IfNotPresent"` |  |
| images.vector.repository | string | `"docker.io/timberio/vector"` |  |
| images.vector.tag | string | `"0.56.0-distroless-static"` |  |
| migration.enabled | bool | `true` |  |
| nameOverride | string | `"krci-audit"` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| replicaCount | int | `1` |  |
| resources.kubeAuditRest.limits.cpu | string | `"1"` |  |
| resources.kubeAuditRest.limits.memory | string | `"32Mi"` |  |
| resources.kubeAuditRest.requests.cpu | string | `"2m"` |  |
| resources.kubeAuditRest.requests.memory | string | `"10Mi"` |  |
| resources.vector.limits.cpu | string | `"1"` |  |
| resources.vector.limits.memory | string | `"256Mi"` |  |
| resources.vector.requests.cpu | string | `"2m"` |  |
| resources.vector.requests.memory | string | `"10Mi"` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| sharedUID | int | `255999` |  |
| tls.createSelfSignedIssuer | bool | `true` |  |
| tls.issuerRef.kind | string | `"Issuer"` |  |
| tls.issuerRef.name | string | `"krci-audit-selfsigned"` |  |
| tolerations | list | `[]` |  |
