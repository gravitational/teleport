# Terraform Infrastructure Makefile

.DEFAULT_GOAL := help

ROOT_MODULE := root-sample

## Run all checks (init, fmt, lint, validate)
.PHONY: all
all: init fmt lint validate

## Initialize Terraform in all directories
.PHONY: init
init:
	terraform -chdir=$(ROOT_MODULE) init

## Format all Terraform files
.PHONY: fmt
fmt:
	terraform fmt -recursive .

## Check formatting without applying changes
.PHONY: fmt-check
fmt-check:
	terraform fmt -check -recursive

## Run tflint on all modules
.PHONY: lint
lint:
	tflint --recursive

## Validate all Terraform configurations
.PHONY: validate
validate:
	terraform -chdir=$(ROOT_MODULE) validate

## Plan infrastructure changes for all root modules
.PHONY: plan
plan:
	terraform -chdir=$(ROOT_MODULE) plan

## Apply infrastructure changes for all root modules
.PHONY: apply
apply:
	terraform -chdir=$(ROOT_MODULE) apply -auto-approve

## Destroy infrastructure for all root modules
.PHONY: destroy
destroy:
	terraform -chdir=$(ROOT_MODULE) destroy -auto-approve

## Destroy and re-apply infrastructure for all root modules
.PHONY: reapply
reapply: destroy apply

## Show this help message
.PHONY: help
help:
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk '/^## / { desc = substr($$0, 4); getline; if (/^\.PHONY:/) getline; if (match($$0, /^[a-zA-Z_-]+:/)) { target = substr($$0, 1, index($$0, ":")-1); printf "  %-20s %s\n", target, desc } }' $(MAKEFILE_LIST) | sort
