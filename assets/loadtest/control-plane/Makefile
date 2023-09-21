# initializes helm repos and install teleport
.PHONY: deploy
deploy:
	./init.sh
	./apply.sh

# initializes helm repos and install teleport
.PHONY: destroy
destroy:
	./clean-non-kube.sh

.PHONY: delete-teleport
delete-teleport:
	./teleport/uninstall-teleport.sh

.PHONY: deploy-teleport
deploy-teleport:
	./teleport/install-teleport.sh
	./teleport/wait.sh auth
	./dns/update-record.sh UPSERT
	./teleport/wait.sh proxy

# forwards the in cluster grafana to localhost:6060
.PHONY: forward-monitor
forward-monitor:
	./monitoring/port-forward.sh

.PHONY: enable-database-agents-monitoring
enable-database-agents-monitoring:
	./monitoring/create-database-agents.sh