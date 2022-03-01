MAKEGO := make/go
MAKEGO_REMOTE := https://github.com/bufbuild/makego.git
PROJECT := buf-push-action
GO_MODULE := github.com/bufbuild/buf-push-action
DOCKER_ORG := bufbuild
DOCKER_PROJECT := buf-push-action

include make/buf-push-action/all.mk
