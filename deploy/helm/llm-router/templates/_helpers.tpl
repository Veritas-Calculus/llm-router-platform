{{/*
Expand the name of the chart.
*/}}
{{- define "llm-router.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "llm-router.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "llm-router.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "llm-router.labels" -}}
helm.sh/chart: {{ include "llm-router.chart" . }}
{{ include "llm-router.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "llm-router.selectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-router.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "llm-router.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "llm-router.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Resolve the PostgreSQL host.
When the subchart is enabled, use its auto-generated service name.
When disabled, fall back to config.dbHost.
*/}}
{{- define "llm-router.dbHost" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" (include "llm-router.fullname" .) }}
{{- else }}
{{- .Values.config.dbHost }}
{{- end }}
{{- end }}

{{/*
Resolve the PostgreSQL user and database from subchart or config.
*/}}
{{- define "llm-router.dbUser" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.username | default "postgres" }}
{{- else }}
{{- .Values.config.dbUser }}
{{- end }}
{{- end }}

{{- define "llm-router.dbName" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.database | default "llm_router" }}
{{- else }}
{{- .Values.config.dbName }}
{{- end }}
{{- end }}

{{/*
Resolve the Redis host.
Bitnami Redis standalone uses "<release>-redis-master" as service name.
*/}}
{{- define "llm-router.redisHost" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-redis-master" (include "llm-router.fullname" .) }}
{{- else }}
{{- .Values.config.redisHost }}
{{- end }}
{{- end }}
