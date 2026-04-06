.PHONY: build

build:
	docker build -t cepheus-probe-agent:latest -f docker/clab/probe-agent.Dockerfile .