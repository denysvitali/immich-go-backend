{{/*
Expand the name of the chart.
*/}}
{{- define "immich-go-backend.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "immich-go-backend.fullname" -}}
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
{{- define "immich-go-backend.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "immich-go-backend.labels" -}}
helm.sh/chart: {{ include "immich-go-backend.chart" . }}
{{ include "immich-go-backend.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "immich-go-backend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "immich-go-backend.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "immich-go-backend.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "immich-go-backend.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference
*/}}
{{- define "immich-go-backend.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Secret name for app secrets (created or existing)
*/}}
{{- define "immich-go-backend.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- include "immich-go-backend.fullname" . }}
{{- end }}
{{- end }}

{{/*
PVC name
*/}}
{{- define "immich-go-backend.pvcName" -}}
{{- if .Values.persistence.existingClaim }}
{{- .Values.persistence.existingClaim }}
{{- else }}
{{- include "immich-go-backend.fullname" . }}-data
{{- end }}
{{- end }}

{{/*
Whether chart should create the Secret resource
*/}}
{{- define "immich-go-backend.createSecret" -}}
{{- if .Values.existingSecret }}
{{- false }}
{{- else }}
{{- true }}
{{- end }}
{{- end }}
