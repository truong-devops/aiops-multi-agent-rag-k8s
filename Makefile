SHELL := /bin/sh

.PHONY: help tree doctor status

help:
	@echo "Available targets:"
	@echo "  make tree    - Show project tree without .git"
	@echo "  make doctor  - Check common local tools"
	@echo "  make status  - Show git status"

tree:
	@if command -v tree >/dev/null 2>&1; then \
		tree -a -I '.git|node_modules|.venv|dist|build|.next'; \
	else \
		find . -path './.git' -prune -o -print | sort; \
	fi

doctor:
	@for tool in git docker kubectl kustomize go python3 node npm; do \
		if command -v $$tool >/dev/null 2>&1; then \
			printf "%-10s %s\n" "$$tool" "OK"; \
		else \
			printf "%-10s %s\n" "$$tool" "missing"; \
		fi; \
	done

status:
	@git status --short
