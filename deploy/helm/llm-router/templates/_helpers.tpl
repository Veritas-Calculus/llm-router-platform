{{/*
Expand the name of the chart.
*/}}
{{- define "llm-router.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
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
Common labels
*/}}
{{- define "llm-router.labels" -}}
helm.sh/chart: {{ include "llm-router.name" . }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "llm-router.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
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
Database DSN
*/}}
{{- define "llm-router.databaseDSN" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "host=%s-postgresql port=5432 user=%s password=%s dbname=%s sslmode=disable" (include "llm-router.fullname" .) .Values.postgresql.auth.username .Values.postgresql.auth.password .Values.postgresql.auth.database }}
{{- else }}
{{- printf "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s" .Values.postgresql.external.host (int .Values.postgresql.external.port) .Values.postgresql.external.username .Values.postgresql.external.password .Values.postgresql.external.database .Values.postgresql.external.sslMode }}
{{- end }}
{{- end }}

{{/*
Redis URL
*/}}
{{- define "llm-router.redisURL" -}}
{{- if .Values.redis.enabled }}
{{- printf "redis://:%s@%s-redis-master:6379/0" .Values.redis.auth.password (include "llm-router.fullname" .) }}
{{- else }}
{{- printf "redis://:%s@%s:%d/0" .Values.redis.external.password .Values.redis.external.host (int .Values.redis.external.port) }}
{{- end }}
{{- end }}
