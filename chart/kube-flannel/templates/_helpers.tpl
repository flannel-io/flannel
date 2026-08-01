{{- define "flannel.name" -}}
{{- .Chart.Name  | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "flannel.selectorLabels" -}}
app: "flannel"
{{- end -}}

{{- define "flannel.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Labels common to all resources */}}
{{- define "flannel.labels" -}}
tier: "node"
helm.sh/chart: {{ include "flannel.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/name: {{ include "flannel.name" . }}
app.kubernetes.io/instance: {{ .Release.Name | trunc 63 | trimSuffix "-" }}
{{ include "flannel.selectorLabels" . }}
{{- with .Values.global.commonLabels }}
{{ toYaml . }}
{{- end -}}
{{- end -}}
