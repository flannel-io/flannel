{{- define "flannel.selectorLabels" -}}
app: "flannel"
tier: "node"
{{- end -}}

{{/* Labels common to all resources */}}
{{- define "flannel.labels" -}}
{{ include "flannel.selectorLabels" . }}
{{- with .Values.global.commonLabels }}
{{ toYaml . }}
{{- end -}}
{{- end -}}
