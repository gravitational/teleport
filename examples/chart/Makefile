include ../../common.mk

.PHONY: render-chart-ref
render-chart-ref:
	$(HELMJANITOR) reference --root-dir="../.."

.PHONY: render-chart-ref-%
render-chart-ref-%:
	$(HELMJANITOR) reference --charts=$* --root-dir="../.."

.PHONY: check-chart-ref
check-chart-ref:
	$(HELMJANITOR) reference --root-dir="../.." --check

.PHONY: check-chart-ref-%
check-chart-ref-%:
	$(HELMJANITOR) reference --charts=$* --root-dir="../.." --check

.PHONY: test
test:
	$(HELMJANITOR) test --root-dir="../.."

.PHONY: test-%
test-%:
	$(HELMJANITOR) test --charts=$* --root-dir="../.."

.PHONY: lint
lint:
	$(HELMJANITOR) lint --root-dir="../.."

.PHONY: lint-%
lint-%:
	$(HELMJANITOR) lint --charts=$* --root-dir="../.."
