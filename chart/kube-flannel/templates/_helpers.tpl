{{/* Labels common to all resources */}}
{{- define "flannel.labels" -}}
app: {{ .Chart.Name | quote }}
tier: node
{{- with .Values.global.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}
