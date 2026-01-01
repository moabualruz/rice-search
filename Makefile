.PHONY: install up down test api-logs worker-logs

install:
	cd backend && pip install -e .[dev]
	cd frontend && npm install

up:
	docker compose -f deploy/docker-compose.yml up -d --build

down:
	docker compose -f deploy/docker-compose.yml down

logs:
	docker compose -f deploy/docker-compose.yml logs -f

api-logs:
	docker compose -f deploy/docker-compose.yml logs -f backend-api

worker-logs:
	docker compose -f deploy/docker-compose.yml logs -f backend-worker

test:
	cd backend && pytest

e2e:
	docker compose -f deploy/docker-compose.yml run --rm tests-e2e
