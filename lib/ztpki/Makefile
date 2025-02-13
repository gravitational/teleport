# v2.4.1
OAPI_CODEGEN_VERSION=9c09ef9e9d4be639bd3feff31ff2c06961421272

.PHONY: gen
gen:
	curl -sSf https://ztpki.venafi.com/api/v2/swagger/swagger.json | jq --sort-keys > swagger.json
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config codegen.yaml swagger.json
