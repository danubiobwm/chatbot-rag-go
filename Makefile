.PHONY: up down test lint run-api run-worker fmt

up: ## sobe LocalStack + OpenSearch
	docker compose up -d

down: ## derruba o ambiente local
	docker compose down -v

test: ## roda todos os testes com cobertura
	go test ./... -race -cover

lint: ## roda go vet + staticcheck (instale com: go install honnef.co/go/tools/cmd/staticcheck@latest)
	go vet ./...
	staticcheck ./...

fmt: ## formata o código
	gofmt -s -w .
	go vet ./...

run-api: ## roda a API localmente (variáveis de ambiente em .env)
	go run ./cmd/api

run-worker: ## roda o worker de processamento localmente
	go run ./cmd/worker
