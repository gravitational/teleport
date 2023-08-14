# deploys etcd into the cluster
.PHONY: deploy
deploy:
	kubectl create namespace etcd --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret tls client-certs -n etcd \
	  --cert=./certs/client-cert.pem \
	  --key=./certs/client-key.pem

	kubectl create secret tls server-certs -n etcd \
  	  --cert=./certs/server-cert.pem \
  	  --key=./certs/server-key.pem

	kubectl create secret tls client-certs -n etcd \
		  --cert=./certs/client-cert.pem \
		  --key=./certs/client-key.pem

	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm install etcd -n etcd --create-namespace bitnami/etcd \
		--set replicaCount=3 \
		--set metrics.enabled=true \
		--set metrics.podMonitor.enabled=true \
		--set auth.peer.secureTransport=true \
		--set auth.peer.useAutoTLS=true \
		--set auth.client.secureTransport=true \
		--set auth.client.existingSecret=client-certs \
		--set auth.client.enableAuthentication=true


# deletes etcd from the cluster
.PHONY: delete
delete:
	helm delete -n etcd etcd
	kubectl delete ns etcd