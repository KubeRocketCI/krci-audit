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
| api | object | `{"enabled":true,"port":8080,"replicaCount":1,"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"32Mi"}}}` | Read-only audit API (initiator lookup + audit-events query). Runs as a separate Deployment and connects to PostgreSQL as the least-privilege audit_reader role, so it can never mutate the trail. Consumers reach it by in-cluster DNS at <release>-api.<namespace>:<api.port>. |
| capture | object | `{"failurePolicy":"Ignore","filter":{"groupResources":[{"group":"tekton.dev","resource":"pipelineruns"}],"groups":["v2.edp.epam.com"]},"includeSubresources":[],"level":"metadata","namespaces":[],"operations":["CREATE","UPDATE","DELETE"],"rules":[{"apiGroups":["v2.edp.epam.com"],"apiVersions":["*"],"resources":["*"],"scope":"*"},{"apiGroups":["tekton.dev"],"apiVersions":["*"],"resources":["pipelineruns"],"scope":"Namespaced"}],"storeRaw":false,"timeoutSeconds":1}` | Capture configuration. The webhook is wildcard-capable; the Vector `filter` selects which events are STORED, and can be changed at any time (edit values + `helm upgrade`) with no code change or rebuild. Defaults select KubeRocketCI objects. |
| capture.failurePolicy | string | `"Ignore"` | never block platform mutations if auditing is down |
| capture.filter | object | `{"groupResources":[{"group":"tekton.dev","resource":"pipelineruns"}],"groups":["v2.edp.epam.com"]}` | Vector store filter (which of the received events are persisted). Default = KRCI objects. |
| capture.filter.groupResources | list | `[{"group":"tekton.dev","resource":"pipelineruns"}]` | Store events matching a specific group+resource. |
| capture.filter.groups | list | `["v2.edp.epam.com"]` | Store any event whose request.resource.group is in this list. |
| capture.includeSubresources | list | `[]` | Subresource policy: spec-level only by default. status/scale churn is dropped unless the subresource name is listed here (and a matching <resource>/<sub> webhook rule is added). |
| capture.level | string | `"metadata"` | Object body capture level: "metadata" (default; bounds size/PII) or "full". |
| capture.namespaces | list | `[]` | Restrict which namespaces are audited, by name. Empty (default) = all namespaces. When set, the apiserver skips the webhook call entirely for every other namespace (cheaper than filtering downstream) — relies on the apiserver's automatic kubernetes.io/metadata.name namespace label (k8s >=1.21). |
| capture.rules | list | `[{"apiGroups":["v2.edp.epam.com"],"apiVersions":["*"],"resources":["*"],"scope":"*"},{"apiGroups":["tekton.dev"],"apiVersions":["*"],"resources":["pipelineruns"],"scope":"Namespaced"}]` | Webhook rules registered with the API server (which admissions are sent to us). |
| capture.storeRaw | bool | `false` | Store the whole raw AdmissionReview payload for replay fidelity (off by default). |
| db.host | string | `""` | Logical database. For pgo/simple the chart provisions this DB; host/port default to the provisioned instance and rarely need overriding. For external, set host (name/port too). required for external |
| db.mode | string | `"external"` | Provisioning mode — choose how PostgreSQL is supplied:   external — bring your own DB (nothing is provisioned; set db.host + db.owner.secretName)   pgo      — provision a Crunchydata PostgresCluster (requires the postgres-operator add-on)   simple   — provision a single plain PostgreSQL Deployment (dev / small installs) |
| db.name | string | `"krci-audit"` |  |
| db.owner | object | `{"passwordKey":"password","secretName":"","userKey":"user"}` | All DB credentials are a prerequisite: create the Secret(s) yourself before installing (e.g. `kubectl create secret generic ...`, or enable `eso` below). The chart only ever reads them.  Default (secretName unset): a single pre-created Secret named <release>-db-access with keys db-owner-username, db-owner-password (simple mode only), writer-password, reader-password (only if api.enabled).  secretName overrides let owner/writer/reader each point at their own existing Secret with custom key names instead (owner.userKey/passwordKey, writer.passwordKey, reader.passwordKey). pgo mode ignores owner.secretName: it reads the operator's own pguser Secret (keys user/password). |
| db.owner.secretName | string | `""` | if not defined: <release>-db-access (simple mode); required for external |
| db.pgo | object | `{"backups":{"enabled":true,"image":"registry.developers.crunchydata.com/crunchydata/crunchy-pgbackrest:ubi8-2.51-0","storage":"2Gi"},"image":"registry.developers.crunchydata.com/crunchydata/crunchy-postgres:ubi8-16.4-0","postgresVersion":16,"replicas":1,"storage":"2Gi"}` | pgo mode (Crunchydata postgres-operator). Provisions a PostgresCluster + owner user. |
| db.port | int | `5432` |  |
| db.reader.passwordKey | string | `"password"` |  |
| db.reader.secretName | string | `""` | if not defined: <release>-db-access |
| db.simple | object | `{"image":"postgres:16-alpine","persistence":true,"resources":{"limits":{"cpu":"1","memory":"512Mi"},"requests":{"cpu":"50m","memory":"128Mi"}},"storage":"2Gi"}` | simple mode. A single PostgreSQL Deployment (no HA, no backups) — suitable for dev. |
| db.simple.persistence | bool | `true` | false ⇒ emptyDir (ephemeral) |
| db.sslmode | string | `"disable"` |  |
| db.writer.passwordKey | string | `"password"` |  |
| db.writer.secretName | string | `""` | if not defined: <release>-db-access |
| eso | object | `{"apiVersion":"external-secrets.io/v1","aws":{"region":"","roleArn":""},"enabled":false,"secretPath":""}` | External Secrets Operator (AWS only): populates the <release>-db-access Secret (see db.* above) from an AWS Parameter Store JSON value at secretPath containing whichever of db-owner-username/db-owner-password/writer-password/reader-password apply to your db.mode. |
| eso.aws.roleArn | string | `""` | IRSA role ARN, required when enabled |
| eso.secretPath | string | `""` | required when enabled: AWS Parameter Store path to the JSON value |
| fullnameOverride | string | `"krci-audit"` |  |
| image | object | `{"pullPolicy":"IfNotPresent","repository":"epamedp/krci-audit","tag":""}` | krci-audit app image: one image ships both binaries (krci-audit-migrate, krci-audit-api); the migration Job and read API Deployment select which one runs via `command`. Top-level (not under `images`) because this is the image the CD Pipeline Operator builds and promotes — it overrides Argo CD Helm parameters image.repository/image.tag/image.digest by convention (see edp-cd-pipeline-operator), so this chart must expose them at this exact path. |
| image.tag | string | `""` | defaults to .Chart.AppVersion |
| imagePullSecrets | list | `[]` |  |
| images.kubeAuditRest | object | `{"pullPolicy":"IfNotPresent","repository":"ghcr.io/richardoc/kube-audit-rest","tag":"ad68f71978e8cd610b5b06769fab301cf9ee74d0-distroless@sha256:2444c1207156681c4ed04e7bb02662820c9bfb31b50e8fe5b0112b3f8f577d42"}` | kube-audit-rest: the ValidatingWebhook that logs the raw AdmissionReview. Pin by digest. |
| images.vector | object | `{"pullPolicy":"IfNotPresent","repository":"docker.io/timberio/vector","tag":"0.56.0-distroless-static"}` | Vector: tails the log and writes to Postgres. >=0.46 for the postgres sink; >=0.53 for TLS. |
| migration | object | `{"enabled":true}` | Schema migration Job (Helm hook on install/upgrade). |
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
| sharedUID | int | `255999` | Both containers must share a uid: kube-audit-rest (lumberjack) writes the log file mode 0600 and the Vector sidecar reads it from the shared volume. |
| tls | object | `{"createSelfSignedIssuer":true,"issuerRef":{"kind":"Issuer","name":"krci-audit-selfsigned"}}` | TLS serving cert for the webhook, cert-manager-managed. The webhook's caBundle is injected by cert-manager via inject-ca-from. |
| tls.createSelfSignedIssuer | bool | `true` | Create a self-signed Issuer in this namespace. Set false to use an existing issuer. |
| tolerations | list | `[]` |  |
