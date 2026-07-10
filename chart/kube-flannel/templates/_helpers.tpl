{{- define "flannel.name" -}}
{{- .Chart.Name  | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "flannel.selectorLabels" -}}
app: "flannel"
tier: "node"
app.kubernetes.io/name: {{ include "flannel.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "flannel.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Labels common to all resources */}}
{{- define "flannel.labels" -}}
helm.sh/chart: {{ include "flannel.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{ include "flannel.selectorLabels" . }}
{{- with .Values.global.commonLabels }}
{{ toYaml . }}
{{- end -}}
{{- end -}}
