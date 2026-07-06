{{/*
Expand the name of the chart.
*/}}
{{- define "krci-audit.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "krci-audit.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "krci-audit.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "krci-audit.labels" -}}
helm.sh/chart: {{ include "krci-audit.chart" . }}
{{ include "krci-audit.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "krci-audit.selectorLabels" -}}
app.kubernetes.io/name: {{ include "krci-audit.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Read API identity. The API runs as a separate Deployment/Pod from the capture pod, so it
needs its own selector (a distinct app.kubernetes.io/name) — the capture Service selects on
the base name and must never route to API pods, and vice versa.
*/}}
{{- define "krci-audit.apiName" -}}
{{- printf "%s-api" (include "krci-audit.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "krci-audit.apiFullname" -}}
{{- printf "%s-api" (include "krci-audit.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "krci-audit.apiSelectorLabels" -}}
app.kubernetes.io/name: {{ include "krci-audit.apiName" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "krci-audit.apiLabels" -}}
helm.sh/chart: {{ include "krci-audit.chart" . }}
{{ include "krci-audit.apiSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: read-api
{{- end }}

{{- define "krci-audit.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "krci-audit.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Name of the serving-cert Secret (written by cert-manager, mounted by kube-audit-rest).
*/}}
{{- define "krci-audit.certSecretName" -}}
{{- printf "%s-tls" (include "krci-audit.fullname" .) }}
{{- end }}

{{/*
Database provisioning helpers. Three modes:
  external — bring your own PostgreSQL; owner creds in db.owner.secretName.
  pgo      — provision a Crunchydata PostgresCluster; owner creds in its pguser Secret.
  simple   — provision a plain PostgreSQL Deployment; owner creds in the prerequisite db-access Secret.
DB credentials are always a prerequisite (pre-created Secret or ESO) — the chart only reads them.
The migration Job then sets the audit_writer LOGIN password from that Secret in every mode.
*/}}

{{- define "krci-audit.pgoClusterName" -}}
{{- include "krci-audit.fullname" . }}
{{- end }}

{{/* DB host reachable in-cluster for the effective mode. */}}
{{- define "krci-audit.dbHost" -}}
{{- if eq .Values.db.mode "pgo" -}}
{{- printf "%s-primary" (include "krci-audit.pgoClusterName" .) -}}
{{- else if eq .Values.db.mode "simple" -}}
{{- printf "%s-db" (include "krci-audit.fullname" .) -}}
{{- else -}}
{{- required "db.host is required when db.mode=external" .Values.db.host -}}
{{- end -}}
{{- end }}

{{/*
Single prerequisite Secret (<release>-db-access) holding whichever of the owner/writer/reader
credentials aren't pointed at an externally-managed Secret. Pre-create it (or populate it via
ESO) before installing — the chart only reads it. Keys: db-owner-username,
db-owner-password (simple mode only), writer-password, reader-password.
*/}}
{{- define "krci-audit.dbAccessSecretName" -}}
{{- printf "%s-db-access" (include "krci-audit.fullname" .) -}}
{{- end }}

{{/* Secret holding the schema-owner user/password. */}}
{{- define "krci-audit.ownerSecretName" -}}
{{- if eq .Values.db.mode "pgo" -}}
{{- printf "%s-pguser-%s" (include "krci-audit.pgoClusterName" .) (include "krci-audit.pgoClusterName" .) -}}
{{- else if eq .Values.db.mode "simple" -}}
{{- include "krci-audit.dbAccessSecretName" . -}}
{{- else -}}
{{- required "db.owner.secretName is required when db.mode=external" .Values.db.owner.secretName -}}
{{- end -}}
{{- end }}

{{- define "krci-audit.ownerUserKey" -}}
{{- if eq .Values.db.mode "simple" -}}db-owner-username{{- else -}}{{ .Values.db.owner.userKey }}{{- end -}}
{{- end }}

{{- define "krci-audit.ownerPasswordKey" -}}
{{- if eq .Values.db.mode "simple" -}}db-owner-password{{- else -}}{{ .Values.db.owner.passwordKey }}{{- end -}}
{{- end }}

{{/*
Owner DB connection env vars sourced from the schema-owner Secret. Shared by the migration Job
and the retention CronJob — both connect as the owner, so the wiring lives in one place.
*/}}
{{- define "krci-audit.ownerDBEnv" -}}
- name: PGHOST
  value: {{ include "krci-audit.dbHost" . | quote }}
- name: PGPORT
  value: {{ .Values.db.port | quote }}
- name: PGDATABASE
  value: {{ .Values.db.name | quote }}
- name: PGSSLMODE
  value: {{ .Values.db.sslmode | quote }}
- name: PGUSER
  valueFrom:
    secretKeyRef:
      name: {{ include "krci-audit.ownerSecretName" . }}
      key: {{ include "krci-audit.ownerUserKey" . }}
- name: PGPASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "krci-audit.ownerSecretName" . }}
      key: {{ include "krci-audit.ownerPasswordKey" . }}
{{- end }}

{{/* Secret holding the audit_writer password. Prerequisite db-access Secret unless overridden. */}}
{{- define "krci-audit.writerSecretName" -}}
{{- default (include "krci-audit.dbAccessSecretName" .) .Values.db.writer.secretName -}}
{{- end }}

{{- define "krci-audit.writerPasswordKey" -}}
{{- if .Values.db.writer.secretName -}}{{ .Values.db.writer.passwordKey }}{{- else -}}writer-password{{- end -}}
{{- end }}

{{/* Secret holding the audit_reader password. Prerequisite db-access Secret unless overridden. */}}
{{- define "krci-audit.readerSecretName" -}}
{{- default (include "krci-audit.dbAccessSecretName" .) .Values.db.reader.secretName -}}
{{- end }}

{{- define "krci-audit.readerPasswordKey" -}}
{{- if .Values.db.reader.secretName -}}{{ .Values.db.reader.passwordKey }}{{- else -}}reader-password{{- end -}}
{{- end }}

