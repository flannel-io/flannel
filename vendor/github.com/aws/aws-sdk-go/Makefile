LINTIGNOREDOT='internal/features.+should not use dot imports'
LINTIGNOREDOC='service/[^/]+/(api|service)\.go:.+(comment on exported|should have comment or be unexported)'
LINTIGNORECONST='service/[^/]+/(api|service)\.go:.+(type|struct field|const|func) ([^ ]+) should be ([^ ]+)'

help:
	@echo "Please use \`make <target>' where <target> is one of"
	@echo "  api_info                to print a list of services and versions"
	@echo "  build                   to go build the SDK"
	@echo "  deps                    to go get the SDK dependencies"
	@echo "  docs                    to build SDK documentation"
	@echo "  generate                to go generate and make services"
	@echo "  generate-protocol-test  to generate protocol tests"
	@echo "  integration             to run integration tests"
	@echo "  lint                    to lint the SDK"
	@echo "  services                to generate services"
	@echo "  unit                    to run unit tests"

default: generate

generate-protocol-test:
	go generate ./internal/protocol/...

generate-test: generate-protocol-test

generate:
	go generate ./internal/endpoints
	@make services

services:
	go generate ./service

integration: deps
	go test ./internal/test/integration/... -tags=integration
	gucumber ./internal/features/smoke

lint: deps
	@echo "golint ./..."
	@lint=`golint ./...`; \
	lint=`echo "$$lint" | grep -E -v -e ${LINTIGNOREDOT} -e ${LINTIGNOREDOC} -e ${LINTIGNORECONST}`; \
	echo "$$lint"; \
	if [ "$$lint" != "" ]; then exit 1; fi

unit: deps build lint
	go test ./...

build:
	go build ./...

docs:
	rm -rf doc && bundle install && bundle exec yard

deps:
	@go get ./...
	@go get github.com/lsegal/gucumber/cmd/gucumber
	@go get github.com/golang/lint/golint

api_info:
	@go run internal/model/cli/api-info/api-info.go

perf:
	@go test -bench . -benchmem -tags 'perf' ./internal/test/perf/...