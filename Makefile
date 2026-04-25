.PHONY: build dist ips apply db dev build-vm telemetry

# Host Related
build:
	docker build -t cepheus-server:latest -f docker/cepheus-server.Dockerfile .
	docker build -t cepheus-stamp-processor:latest -f docker/cepheus-stamp-processor.Dockerfile .
	docker build -t cepheus-trace-processor:latest -f docker/cepheus-trace-processor.Dockerfile .

dev: build
	docker compose up --build cepheus-server cepheus-stamp-processor cepheus-trace-processor

db:
	docker compose up --build pgadmin db nats-server

telemetry:
	docker compose up otel-collector grafana loki tempo

# VM Related
build-vm:
	docker build --output type=local,dest=dist/ -f docker/cepheus-agent.build.Dockerfile .
	docker build --output type=local,dest=dist/ -f docker/scamper.build.Dockerfile .
	docker build -t cepheus-sa:latest -f docker/dev/clab/security-appliance.Dockerfile .

clean:
	-sudo containerlab destroy -t clab/small-retail-store.clab.yaml
	-sudo docker rm -f $$(docker ps -aq --filter "name=^clab-retail-")

ips:
	@printf "%-24s %-60s\n" "Name" "Interfaces"
	@for c in $$(docker ps --format '{{.Names}}' | grep '^clab-retail-'); do \
	  ifaces=$$(docker exec -it "$$c" sh -c "ip -4 -o addr show scope global | awk '\$$2!=\"lo\" && \$$2!=\"eth0\" {print \$$2 \":\" \$$4}'" 2>/dev/null | tr -d '\r' | paste -sd ', ' -); \
	  printf "%-24s %-60s\n" "$$c" "$${ifaces:-<none>}"; \
	done

apply: clean build-vm
	sudo clab deploy -t clab/small-retail-store.clab.yaml
	$(MAKE) ips


# Tests
test.build:
	docker build -t test-stamp-suite:latest -f docker/tests/stamp.test.Dockerfile .
	docker build -t test-scamper-suite:latest -f docker/tests/scamper.test.Dockerfile .
	docker build --output type=local,dest=dist/ -f docker/scamper.build.Dockerfile .

test.integration: test.build
	docker compose -f docker-compose.test.yaml up -d
	go test -v -tags integration ./stamp/tests/integration
	docker compose -f docker-compose.test.yaml exec scamper-test-suite go test -v -tags integration /app/scamper/tests/integration
	docker compose -f docker-compose.test.yaml down

# Generators
sqlc-gen:
	UID=$(shell id -u) GID=$(shell id -g) docker compose -f docker-compose.dev.yaml run --rm sqlc