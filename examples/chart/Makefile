# TODO(hugoShaka): uncomment the additional targets as we start sync-ing
# the reference and the values.yaml

access = discord email jira mattermost msteams pagerduty slack

check_access = $(addprefix check-chart-ref-access-,$(access))
render_access = $(addprefix render-chart-ref-access-,$(access))

.PHONY: render-chart-ref
render-chart-ref: render-chart-ref-example render-chart-ref-teleport-operator render-chart-ref-teleport-kube-agent $(render_access)  # render-chart-ref-teleport-cluster

.PHONY: render-chart-ref-example
render-chart-ref-example:
	cd ../../build.assets/tooling && \
		go run ./cmd/render-helm-ref -chart ./cmd/render-helm-ref/testdata -output ./cmd/render-helm-ref/testdata/expected-output.mdx

# .PHONY: render-chart-ref-teleport-cluster
# render-chart-ref-teleport-cluster:
# 	cd ../../build.assets/tooling && \
# 	go run ./cmd/render-helm-ref -chart ../../examples/chart/teleport-cluster/ -output ../../docs/pages/reference/helm-reference/includes/zz_generated.teleport-cluster.mdx
#
#
.PHONY: render-chart-ref-teleport-kube-agent
render-chart-ref-teleport-kube-agent:
	cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ../../examples/chart/teleport-kube-agent/ -output ../../docs/pages/reference/helm-reference/includes/zz_generated.teleport-kube-agent.mdx

.PHONY: render-chart-ref-teleport-operator
render-chart-ref-teleport-operator:
	cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ../../examples/chart/teleport-cluster/charts/teleport-operator -output ../../docs/pages/reference/helm-reference/includes/zz_generated.teleport-operator.mdx

.PHONY: render-chart-ref-access-%
render-chart-ref-access-%:
	cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ../../examples/chart/access/$* -output ../../docs/pages/reference/helm-reference/includes/zz_generated.access-$*.mdx

.PHONY: check-chart-ref
check-chart-ref: check-chart-ref-example check-chart-ref-teleport-operator check-chart-ref-teleport-kube-agent $(check_access) #check-chart-ref-teleport-cluster

.PHONY: check-chart-ref-example
check-chart-ref-example:
	@ echo "Checking example chart reference"
	@ cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ./cmd/render-helm-ref/testdata -output - | diff ../../build.assets/tooling/cmd/render-helm-ref/testdata/expected-output.mdx - || \
	( echo "Chart values.yaml and reference differ, please run 'make render-chart-ref'" && exit 1 )

# .PHONY: check-chart-ref-teleport-cluster
# check-chart-ref-teleport-cluster:
# 	echo "Checking teleport-cluster reference"
# 	cd ../../build.assets/tooling && \
# 	go run ./cmd/render-helm-ref -chart ../../examples/chart/teleport-cluster -output - | diff ../../docs/pages/reference/helm-reference/includes/zz_generated.teleport-cluster.mdx - || \
# 	( echo "Chart values.yaml and reference differ, please run 'make render-chart-ref'" && exit 1 )
#
.PHONY: check-chart-ref-teleport-kube-agent
check-chart-ref-teleport-kube-agent:
	@ echo "Checking teleport-kube-agent reference"
	@ cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ../../examples/chart/teleport-kube-agent -output - | diff ../../docs/pages/reference/helm-reference/includes/zz_generated.teleport-kube-agent.mdx - || \
	( echo "Chart values.yaml and reference differ, please run 'make render-chart-ref'" && exit 1 )

.PHONY: check-chart-ref-teleport-operator
check-chart-ref-teleport-operator:
	@echo "Checking teleport-operator reference"
	@ cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ../../examples/chart/teleport-cluster/charts/teleport-operator -output - | diff ../../docs/pages/reference/helm-reference/includes/zz_generated.teleport-operator.mdx - || \
	( echo "Chart values.yaml and reference differ, please run 'make render-chart-ref'" && exit 1 )

.PHONY: check-chart-ref-access-%
check-chart-ref-access-%:
	@echo "Checking access/$* reference"
	@ cd ../../build.assets/tooling && \
	go run ./cmd/render-helm-ref -chart ../../examples/chart/access/$* -output - | diff ../../docs/pages/reference/helm-reference/includes/zz_generated.access-$*.mdx - || \
	( echo "Chart values.yaml and reference differ, please run 'make render-chart-ref'" && exit 1 )
