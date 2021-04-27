{{/*
Expand the name of the chart.
*/}}
{{- define "nebula-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nebula-operator.fullname" -}}
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
Expand the namespace of the chart.
*/}}
{{- define "nebula-operator.namespace" -}}
{{ .Release.Namespace }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nebula-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
The ImagePullSecrets.
*/}}
{{- define "nebula-operator.imagePullSecrets" -}}
{{- if .Values.imagePullSecrets }}
imagePullSecrets:
{{- toYaml .Values.imagePullSecrets | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Nebula operator common selector labels
*/}}
{{- define "nebula-operator.matchLabels" -}}
app.kubernetes.io/name: {{ include "nebula-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Nebula operator common labels
*/}}
{{- define "nebula-operator.labels" -}}
{{ include "nebula-operator.matchLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ include "nebula-operator.chart" . }}
{{- end }}

{{/*
Controller manager name of the chart.
*/}}
{{- define "controller-manager.name" -}}
{{ include "nebula-operator.name" . }}-controller-manager
{{- end }}

{{/*
Controller manager selector labels
*/}}
{{- define "controller-manager.matchLabels" -}}
app.kubernetes.io/component: controller-manager
{{ include "nebula-operator.matchLabels" . }}
{{- end }}

{{/*
Controller manager labels
*/}}
{{- define "controller-manager.labels" -}}
app.kubernetes.io/component: controller-manager
{{ include "nebula-operator.labels" . }}
{{- end }}

{{/*
Admission webhook name of the chart.
*/}}
{{- define "admission-webhook.name" -}}
{{ include "nebula-operator.name" . }}-webhook
{{- end }}

{{/*
Admission webhook selector labels
*/}}
{{- define "admission-webhook.matchLabels" -}}
app.kubernetes.io/component: controller-manager
{{ include "nebula-operator.matchLabels" . }}
{{- end -}}

{{/*
Admission webhook labels
*/}}
{{- define "admission-webhook.labels" -}}
app.kubernetes.io/component: admission-webhook
{{ include "nebula-operator.labels" . }}
{{- end -}}

{{/*
Admission webhook objectSelector
*/}}
{{- define "admission-webhook.objectSelector" -}}
{{- if semverCompare ">=1.15-0" .Capabilities.KubeVersion.Version -}}
objectSelector:
  matchLabels:
    "app.kubernetes.io/managed-by": "nebula-operator"
{{- end -}}
{{- end -}}

{{/*
Controller manager name of the chart.
*/}}
{{- define "scheduler.name" -}}
{{ include "nebula-operator.name" . }}-scheduler
{{- end }}

{{/*
Controller manager selector labels
*/}}
{{- define "scheduler.matchLabels" -}}
app.kubernetes.io/component: scheduler
{{ include "nebula-operator.matchLabels" . }}
{{- end }}

{{/*
Controller manager labels
*/}}
{{- define "scheduler.labels" -}}
app.kubernetes.io/component: scheduler
{{ include "nebula-operator.labels" . }}
{{- end }}