define USAGE

"Go blocks" project. A collection of simple but useful Go packages.

Usage: make <target>

some of the <targets> are:

  lint             - run linters
  test             - run tests
  update-deps      - update dependencies

endef
export USAGE

define CAKE
   \033[1;31m. . .\033[0m
   i i i
  %~%~%~%
  |||||||
-=========-
endef
export CAKE

help:
	@echo "$$USAGE"

test:
	go test -cover ./...

lint:
	golangci-lint run

update-deps:
	go get -u ./... && go mod tidy && go mod vendor

cake:
	@printf "%b\n" "$$CAKE"

.PHONY: help test lint update-deps cake