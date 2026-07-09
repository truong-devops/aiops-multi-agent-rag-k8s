SHELL := /bin/sh

.PHONY: help tree doctor status compose-config compose-up compose-up-aiops compose-down compose-logs test-video test-video-integration test-media test-media-integration test-live smoke-product smoke-media-ffmpeg compose-test-db-up compose-test-db-down compose-media-test-db-up compose-media-test-db-down

help:
	@echo "Available targets:"
	@echo "  make tree    - Show project tree without .git"
	@echo "  make doctor  - Check common local tools"
	@echo "  make status  - Show git status"
	@echo "  make compose-config  - Validate rendered docker-compose configuration"
	@echo "  make compose-up      - Build and run local product stack"
	@echo "  make compose-up-aiops - Build and run local product stack with aiops-service profile"
	@echo "  make compose-down    - Stop local compose stack"
	@echo "  make compose-logs    - Follow local compose logs"
	@echo "  make test-video             - Run video-service tests"
	@echo "  make test-video-integration - Run video-service PostgreSQL integration tests"
	@echo "  make test-media             - Run media-worker tests"
	@echo "  make test-media-integration - Run media-worker PostgreSQL integration tests"
	@echo "  make test-live              - Run live-service tests"
	@echo "  make smoke-product          - Run gateway-level product backend smoke test"
	@echo "  make smoke-media-ffmpeg     - Run media-worker FFmpeg smoke test"

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

compose-config:
	@docker compose config

compose-up:
	@docker compose up --build

compose-up-aiops:
	@docker compose --profile aiops up --build

compose-down:
	@docker compose down

compose-logs:
	@docker compose logs -f

test-video:
	@cd services/video-service && go test ./...

test-media:
	@cd services/media-worker && go test ./...

test-live:
	@cd services/live-service && go test ./...

smoke-product:
	@scripts/smoke/product-smoke.sh

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

compose-media-test-db-up:
	@docker compose --profile test up -d postgres-media-test
	@i=0; \
	until docker compose exec -T postgres-media-test pg_isready -U media -d media_test >/dev/null 2>&1; do \
		i=$$((i + 1)); \
		if [ "$$i" -gt 30 ]; then \
			echo "postgres-media-test did not become ready"; \
			exit 1; \
		fi; \
		sleep 1; \
	done

test-media-integration: compose-media-test-db-up
	@cd services/media-worker && \
	MEDIA_WORKER_TEST_DATABASE_URL="$${MEDIA_WORKER_TEST_DATABASE_URL:-postgres://media:media@localhost:$${POSTGRES_MEDIA_TEST_PORT:-55433}/media_test?sslmode=disable}" \
	go test ./internal/repository

smoke-media-ffmpeg:
	@cd services/media-worker && go test -tags smoke ./internal/processor -run TestFFmpegProcessorSmoke -count=1

compose-test-db-down:
	@docker compose --profile test stop postgres-test >/dev/null
	@docker compose --profile test rm -f postgres-test >/dev/null

compose-media-test-db-down:
	@docker compose --profile test stop postgres-media-test >/dev/null
	@docker compose --profile test rm -f postgres-media-test >/dev/null
