.PHONY: setup-auth
setup-auth:
	$(eval POD := $(shell kubectl get pods --selector=app=teleport -o jsonpath='{.items[*].metadata.name}'))
	kubectl cp ./teleport/admin.yaml $(POD):/tmp/admin.yaml
	kubectl cp ./teleport/connector.yaml $(POD):/tmp/connector.yaml

	kubectl exec -i $(POD) -- /usr/local/bin/tctl create -f /tmp/admin.yaml
	kubectl exec -i $(POD) -- /usr/local/bin/tctl create -f /tmp/connector.yaml
