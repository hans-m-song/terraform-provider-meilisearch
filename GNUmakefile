default: build

build:
	go build ./...

testacc:
	bash scripts/test-acceptance.sh

dev:
	docker compose --file docker_compose/docker-compose.yml up
