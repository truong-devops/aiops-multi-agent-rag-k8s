SHELL := /bin/sh

.PHONY: help tree doctor status test-video test-video-integration compose-test-db-up compose-test-db-down

help:
	@echo "Available targets:"
	@echo "  make tree    - Show project tree without .git"
	@echo "  make doctor  - Check common local tools"
	@echo "  make status  - Show git status"
	@echo "  make test-video             - Run video-service tests"
	@echo "  make test-video-integration - Run video-service PostgreSQL integration tests"

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

test-video:
	@cd services/video-service && go test ./...

compose-test-db-up:
	@docker compose --profile test up -d postgres-test
	@i=0; \
	until docker compose exec -T postgres-test pg_isready -U video -d video_test >/dev/null 2>&1; do \
		i=$$((i + 1)); \
		if [ "$$i" -gt 30 ]; then \
			echo "postgres-test did not become ready"; \
			exit 1; \
		fi; \
		sleep 1; \
	done

test-video-integration: compose-test-db-up
	@cd services/video-service && \
	VIDEO_SERVICE_TEST_DATABASE_URL="$${VIDEO_SERVICE_TEST_DATABASE_URL:-postgres://video:video@localhost:$${POSTGRES_TEST_PORT:-55432}/video_test?sslmode=disable}" \
	go test ./internal/repository

compose-test-db-down:
	@docker compose --profile test stop postgres-test >/dev/null
	@docker compose --profile test rm -f postgres-test >/dev/null
