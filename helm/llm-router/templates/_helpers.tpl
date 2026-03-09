{{/*
Common labels
*/}}
{{- define "llm-router.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Server selector labels
*/}}
{{- define "llm-router.server.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}-server
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Web selector labels
*/}}
{{- define "llm-router.web.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}-web
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Full server image name
*/}}
{{- define "llm-router.server.image" -}}
{{ .Values.server.image.repository }}:{{ .Values.server.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Full web image name
*/}}
{{- define "llm-router.web.image" -}}
{{ .Values.web.image.repository }}:{{ .Values.web.image.tag | default .Chart.AppVersion }}
{{- end }}
