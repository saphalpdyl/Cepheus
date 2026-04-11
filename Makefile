.PHONY: build dist


build:
	docker build --output type=local,dest=dist/ -f docker/cepheus-agent.build.Dockerfile .

clean:
	-sudo containerlab destroy -t clab/small-retail-store.clab.yaml
	-sudo docker rm -f $$(docker ps -aq --filter "name=^clab-retail-")

ips:
	@printf "%-24s %-60s\n" "Name" "Interfaces"
	@for c in $$(docker ps --format '{{.Names}}' | grep '^clab-retail-'); do \
	  ifaces=$$(docker exec -it "$$c" sh -c "ip -4 -o addr show scope global | awk '\$$2!=\"lo\" && \$$2!=\"eth0\" {print \$$2 \":\" \$$4}'" 2>/dev/null | tr -d '\r' | paste -sd ', ' -); \
	  printf "%-24s %-60s\n" "$$c" "$${ifaces:-<none>}"; \
	done

apply: clean build
	sudo clab deploy -t clab/small-retail-store.clab.yaml
	$(MAKE) ips