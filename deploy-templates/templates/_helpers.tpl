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
  simple   — provision a plain PostgreSQL Deployment; owner creds in a chart Secret.
The audit_writer role/password is chart-managed in all modes (the migration Job sets it).
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

{{/* Secret holding the schema-owner user/password (keys: user, password). */}}
{{- define "krci-audit.ownerSecretName" -}}
{{- if eq .Values.db.mode "pgo" -}}
{{- printf "%s-pguser-%s" (include "krci-audit.pgoClusterName" .) (include "krci-audit.pgoClusterName" .) -}}
{{- else if eq .Values.db.mode "simple" -}}
{{- printf "%s-db" (include "krci-audit.fullname" .) -}}
{{- else -}}
{{- required "db.owner.secretName is required when db.mode=external" .Values.db.owner.secretName -}}
{{- end -}}
{{- end }}

{{/* Secret holding the audit_writer password (key: password). Chart-created unless provided. */}}
{{- define "krci-audit.writerSecretName" -}}
{{- default (printf "%s-writer" (include "krci-audit.fullname" .)) .Values.db.writer.secretName -}}
{{- end }}

{{/*
Preserve a chart-managed password across upgrades: prefer an explicit value, then the
already-installed Secret's key, then a freshly generated one.
Args (dict): root (.), secretName, key, explicit.
*/}}
{{- define "krci-audit.persistedPassword" -}}
{{- if .explicit -}}
{{- .explicit -}}
{{- else -}}
{{-   $existing := lookup "v1" "Secret" .root.Release.Namespace .secretName -}}
{{-   if and $existing $existing.data (index $existing.data .key) -}}
{{-     index $existing.data .key | b64dec -}}
{{-   else -}}
{{-     randAlphaNum 24 -}}
{{-   end -}}
{{- end -}}
{{- end }}
