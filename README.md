# Chatbot RAG — Go + AWS (Bedrock, OpenSearch, Textract)

> [Português](#português) | [English](#english)

---

<a name="português"></a>

## Português

Chatbot conversacional com **RAG (Retrieval-Augmented Generation)** que responde perguntas
com base em documentos internos (RH, jurídico, políticas), construído em **Go** seguindo
**Clean Architecture** e princípios **SOLID**, simulando AWS localmente com **LocalStack**.

> Projeto de portfólio — pipeline de ingestão de documentos + busca semântica + bot
> multi-canal (WhatsApp/Slack), com deduplicação, fallback de segmentação, cache de
> respostas e observabilidade.

---

### Índice

- [Arquitetura](#arquitetura)
- [Por que esta estrutura de pastas](#por-que-esta-estrutura-de-pastas)
- [Princípios SOLID aplicados](#princípios-solid-aplicados)
- [Pré-requisitos](#pré-requisitos)
- [Passo a passo — rodando localmente](#passo-a-passo--rodando-localmente)
- [Variáveis de ambiente](#variáveis-de-ambiente)
- [Testes](#testes)
- [Decisões técnicas e trade-offs](#decisões-técnicas-e-trade-offs)
- [Roadmap](#roadmap)

---

### Arquitetura

```
                         ┌────────────────────────────┐
   PDFs (S3) ──────────► │      Pipeline assíncrono    │
                         │  Extração → Segmentação →   │
                         │  Embedding → Indexação       │
                         │  (worker, consome SQS+DLQ)  │
                         └──────────────┬─────────────┘
                                        │
                                  OpenSearch (vetores)
                                        │
   WhatsApp/Slack ───► API HTTP ───► RAG Query ◄────────┘
                       (router)     (busca + cache + LLM)
                          │
                          ▼
                    DynamoDB (sessão c/ TTL)
```

Duas aplicações independentes, cada uma com seu próprio `main.go`:

| App | Caminho | Responsabilidade |
|---|---|---|
| **API** | `cmd/api` | Recebe mensagens via webhook, consulta o RAG, responde no canal de origem |
| **Worker** | `cmd/worker` | Consome a fila de extração, processa documentos (Textract → segmentação → embedding → índice) |

### Por que esta estrutura de pastas

```
chatbot-rag-go/
├── cmd/
│   ├── api/main.go        # composition root da API (DI manual)
│   └── worker/main.go     # composition root do worker (DI manual)
├── internal/
│   ├── domain/            # entidades + interfaces (ports). Não importa nada de fora.
│   ├── usecase/           # regra de negócio pura, depende só de domain
│   ├── infrastructure/
│   │   ├── aws/           # implementações concretas (Textract, Bedrock, DynamoDB, OpenSearch, SQS)
│   │   ├── channels/      # adapters de canal (WhatsApp, Slack)
│   │   └── cache/         # cache de respostas em memória
│   ├── config/            # leitura de variáveis de ambiente
│   └── handler/           # camada HTTP (webhook + healthcheck)
├── pkg/
│   ├── logger/            # abstração de log (zap por trás)
│   └── apperrors/         # erros tipados da aplicação
├── test/
│   ├── mocks/             # mocks das interfaces de domain (testify/mock)
│   └── *_test.go
├── deploy/localstack-init/ # script que cria bucket/filas/tabelas no LocalStack
├── docker-compose.yml      # LocalStack + OpenSearch
└── Makefile
```

A regra é simples e é a mesma usada em times sêniores: **a seta de dependência
sempre aponta para `domain`**.

```
infrastructure ──► usecase ──► domain
handler        ──► usecase ──► domain
cmd            ──► (monta tudo, conhece infra e usecase)
```

`domain` nunca importa `infrastructure`. Isso significa: trocar Bedrock por OpenAI,
ou DynamoDB por Postgres, é uma mudança isolada em um arquivo de `infrastructure/`
— os usecases e os testes não mudam uma linha.

### Princípios SOLID aplicados

| Princípio | Onde aparece no código |
|---|---|
| **S**ingle Responsibility | Cada usecase faz uma coisa: `IngestDocumentUseCase` só decide se ingere ou ignora; `ProcessDocumentUseCase` só orquestra o pipeline; `RAGQueryUseCase` só responde perguntas |
| **O**pen/Closed | Novo canal de chat (Telegram, Teams) = nova struct implementando `ChannelAdapter`, zero mudança em `RouteMessageUseCase` |
| **L**iskov Substitution | Qualquer implementação de `Segmenter` (LLM, fallback, ou as duas combinadas via decorator) pode substituir outra sem quebrar `ProcessDocumentUseCase` |
| **I**nterface Segregation | `domain/ports.go` tem 10 interfaces pequenas (`Embedder`, `VectorStore`, `LLMClient`...) em vez de uma `AWSGateway` gigante |
| **D**ependency Inversion | Usecases recebem interfaces no construtor (`NewRAGQueryUseCase(sessions, embedder, store, llm, cache, log)`); quem decide a implementação concreta é o `main.go` (composition root) |

Padrões adicionais usados de propósito (comuns em entrevista técnica sênior):

- **Decorator** — `LLMSegmenter` decora `FixedSizeSegmenter`: tenta o LLM, cai no
  determinístico se falhar, sem o usecase saber que isso existe.
- **Repository** — `DocumentRepository`, `ChunkRepository`, `SessionRepository` isolam
  persistência da regra de negócio.
- **Composition root** — toda a "fiação" de dependências concretas vive só em
  `cmd/api/main.go` e `cmd/worker/main.go`. Nenhum `New...()` de infraestrutura é
  chamado de dentro de `usecase/`.

### Pré-requisitos

- Go 1.22+
- Docker + Docker Compose
- [awslocal](https://github.com/localstack/awscli-local) (opcional, só pra inspecionar o LocalStack manualmente)
- Conta AWS com acesso ao Bedrock habilitado (só necessário se for usar o LLM/embeddings reais — ver nota abaixo)

> **Nota sobre o Bedrock no LocalStack:** o LocalStack (mesmo na versão paga) não
> simula o Bedrock. Para desenvolvimento 100% local sem custo, troque
> `BedrockEmbedder`/`BedrockLLM` por uma implementação que chama um modelo local
> (ex: Ollama) — como ambos implementam interfaces (`domain.Embedder`,
> `domain.LLMClient`), basta criar `internal/infrastructure/ollama/` e trocar a
> linha no composition root. O restante do pipeline (S3, SQS, DynamoDB, Textract)
> funciona normalmente no LocalStack.

### Passo a passo — rodando localmente

#### 1. Clonar e configurar

```bash
git clone https://github.com/danubiobwm/chatbot-rag-go.git
cd chatbot-rag-go
cp .env.example .env
# edite .env se quiser usar tokens reais de Slack/WhatsApp
export $(cat .env | xargs)
```

#### 2. Subir a infraestrutura local

```bash
make up
```

Isso sobe o LocalStack (S3, SQS com DLQ, DynamoDB, Textract mock) e o OpenSearch.
O script `deploy/localstack-init/init.sh` roda automaticamente e cria:

- bucket `chatbot-regulatorio-docs`
- fila `extraction-queue` com redrive policy para `extraction-queue-dlq` (3 tentativas)
- tabelas `ChatbotDocuments` (com GSI por hash), `ChatbotChunks`, `ChatbotSessions` (com TTL)

Confirme que subiu:

```bash
curl http://localhost:4566/_localstack/health
curl http://localhost:9200
```

#### 3. Criar o índice vetorial no OpenSearch

```bash
curl -X PUT "http://localhost:9200/chatbot-regulatorio" -H "Content-Type: application/json" -d '{
  "settings": { "index": { "knn": true } },
  "mappings": {
    "properties": {
      "document_id": { "type": "keyword" },
      "content": { "type": "text" },
      "embedding": { "type": "knn_vector", "dimension": 1024 }
    }
  }
}'
```

#### 4. Instalar dependências Go

```bash
go mod tidy
```

#### 5. Rodar os testes

```bash
make test
```

#### 6. Rodar a API e o worker

Em dois terminais:

```bash
make run-api      # sobe o webhook em :8080
make run-worker   # começa a consumir a fila de extração
```

#### 7. Simular uma pergunta

```bash
curl -X POST http://localhost:8080/webhook/message \
  -H "Content-Type: application/json" \
  -d '{"channel":"slack","user_id":"U123","text":"Qual a política de home office?"}'
```

Se o Bedrock real estiver configurado (`AWS_REGION` + credenciais válidas, sem
`AWS_ENDPOINT_URL` para essa chamada específica), a resposta vai sair gerada com
base no contexto recuperado do OpenSearch.

### Variáveis de ambiente

Veja `.env.example`. As principais:

| Variável | Descrição |
|---|---|
| `AWS_ENDPOINT_URL` | Se setada, todos os clientes AWS apontam para o LocalStack. Deixe vazia para usar a AWS real |
| `BEDROCK_LLM_MODEL_ID` | Qual modelo do Bedrock usar para geração de respostas |
| `OPENSEARCH_URL` | Endpoint do OpenSearch (local ou domínio gerenciado na AWS) |
| `SLACK_BOT_TOKEN` / `WHATSAPP_TOKEN` | Credenciais dos canais de saída |

### Testes

```bash
make test   # unitários, com -race e cobertura
make lint   # go vet + staticcheck
```

Os usecases são testados com mocks das interfaces de `domain` (`test/mocks`),
sem nenhuma dependência de rede ou AWS — é por isso que valeu a pena investir
nas interfaces pequenas (ISP) desde o início.

### Decisões técnicas e trade-offs

- **DLQ em todas as filas**: na v1 do projeto (baseada só em diagrama), uma falha
  de Textract ou do LLM de segmentação simplesmente perdia o documento. Aqui, depois
  de N tentativas, a mensagem cai na DLQ e pode ser inspecionada/reprocessada.
- **Fallback determinístico na segmentação**: nunca deixamos uma falha de LLM travar
  o pipeline inteiro — ver `LLMSegmenter` (decorator).
- **Deduplicação por hash**: reenviar o mesmo PDF não reprocessa nem reembeda —
  ver `IngestDocumentUseCase.Execute` + GSI `content_hash-index`.
- **Cache de respostas em memória**: suficiente para portfólio/demo; em produção
  com múltiplas instâncias, troque por Redis/ElastiCache — só a implementação de
  `domain.ResponseCache` muda.
- **TTL nativo do DynamoDB nas sessões**: evita job de limpeza manual.

### Roadmap

- [ ] Adapter de canal para Telegram (mostrar o Open/Closed na prática)
- [ ] Métricas Prometheus + tracing OpenTelemetry (substituindo o `Notifier` simples)
- [ ] Implementação `infrastructure/ollama` para rodar sem custo de Bedrock
- [ ] IaC (Terraform) para o ambiente real na AWS, espelhando o `docker-compose`

---

<a name="english"></a>

## English

A conversational chatbot with **RAG (Retrieval-Augmented Generation)** that answers questions
based on internal documents (HR, legal, policies), built in **Go** following
**Clean Architecture** and **SOLID** principles, simulating AWS locally with **LocalStack**.

> Portfolio project — document ingestion pipeline + semantic search + multi-channel bot
> (WhatsApp/Slack), with deduplication, segmentation fallback, response caching, and
> observability.

---

### Table of Contents

- [Architecture](#architecture)
- [Why this folder structure](#why-this-folder-structure)
- [SOLID principles applied](#solid-principles-applied)
- [Prerequisites](#prerequisites)
- [Step by step — running locally](#step-by-step--running-locally)
- [Environment variables](#environment-variables)
- [Tests](#tests)
- [Technical decisions and trade-offs](#technical-decisions-and-trade-offs)
- [Roadmap](#roadmap-1)

---

### Architecture

```
                         ┌────────────────────────────┐
   PDFs (S3) ──────────► │     Async Pipeline          │
                         │  Extract → Segment →        │
                         │  Embed → Index              │
                         │  (worker, consumes SQS+DLQ) │
                         └──────────────┬─────────────┘
                                        │
                                  OpenSearch (vectors)
                                        │
   WhatsApp/Slack ───► HTTP API ───► RAG Query ◄───────┘
                       (router)    (search + cache + LLM)
                          │
                          ▼
                    DynamoDB (session w/ TTL)
```

Two independent applications, each with its own `main.go`:

| App | Path | Responsibility |
|---|---|---|
| **API** | `cmd/api` | Receives messages via webhook, queries RAG, responds on the source channel |
| **Worker** | `cmd/worker` | Consumes the extraction queue, processes documents (Textract → segment → embed → index) |

### Why this folder structure

```
chatbot-rag-go/
├── cmd/
│   ├── api/main.go        # API composition root (manual DI)
│   └── worker/main.go     # worker composition root (manual DI)
├── internal/
│   ├── domain/            # entities + interfaces (ports). Imports nothing external.
│   ├── usecase/           # pure business logic, depends only on domain
│   ├── infrastructure/
│   │   ├── aws/           # concrete implementations (Textract, Bedrock, DynamoDB, OpenSearch, SQS)
│   │   ├── channels/      # channel adapters (WhatsApp, Slack)
│   │   └── cache/         # in-memory response cache
│   ├── config/            # environment variable loading
│   └── handler/           # HTTP layer (webhook + healthcheck)
├── pkg/
│   ├── logger/            # logging abstraction (zap underneath)
│   └── apperrors/         # typed application errors
├── test/
│   ├── mocks/             # mocks for domain interfaces (testify/mock)
│   └── *_test.go
├── deploy/localstack-init/ # script that creates buckets/queues/tables in LocalStack
├── docker-compose.yml      # LocalStack + OpenSearch
└── Makefile
```

The rule is simple and the same used by senior teams: **the dependency arrow
always points to `domain`**.

```
infrastructure ──► usecase ──► domain
handler        ──► usecase ──► domain
cmd            ──► (wires everything, knows both infra and usecase)
```

`domain` never imports `infrastructure`. This means: swapping Bedrock for OpenAI,
or DynamoDB for Postgres, is an isolated change in a single `infrastructure/` file
— usecases and tests don't change a single line.

### SOLID principles applied

| Principle | Where it appears in the code |
|---|---|
| **S**ingle Responsibility | Each usecase does one thing: `IngestDocumentUseCase` only decides whether to ingest or skip; `ProcessDocumentUseCase` only orchestrates the pipeline; `RAGQueryUseCase` only answers questions |
| **O**pen/Closed | New chat channel (Telegram, Teams) = new struct implementing `ChannelAdapter`, zero changes to `RouteMessageUseCase` |
| **L**iskov Substitution | Any `Segmenter` implementation (LLM, fallback, or both combined via decorator) can replace another without breaking `ProcessDocumentUseCase` |
| **I**nterface Segregation | `domain/ports.go` has 10 small interfaces (`Embedder`, `VectorStore`, `LLMClient`...) instead of one giant `AWSGateway` |
| **D**ependency Inversion | Usecases receive interfaces in the constructor (`NewRAGQueryUseCase(sessions, embedder, store, llm, cache, log)`); `main.go` (composition root) decides the concrete implementation |

Additional patterns used intentionally (common in senior technical interviews):

- **Decorator** — `LLMSegmenter` decorates `FixedSizeSegmenter`: tries the LLM, falls
  back to deterministic if it fails, without the usecase knowing this exists.
- **Repository** — `DocumentRepository`, `ChunkRepository`, `SessionRepository` isolate
  persistence from business logic.
- **Composition root** — all concrete dependency wiring lives only in
  `cmd/api/main.go` and `cmd/worker/main.go`. No `New...()` infrastructure calls
  happen inside `usecase/`.

### Prerequisites

- Go 1.22+
- Docker + Docker Compose
- [awslocal](https://github.com/localstack/awscli-local) (optional, only for manually inspecting LocalStack)
- AWS account with Bedrock access enabled (only required if using real LLM/embeddings — see note below)

> **Note about Bedrock on LocalStack:** LocalStack (even the paid version) does not
> simulate Bedrock. For 100% local development at no cost, swap
> `BedrockEmbedder`/`BedrockLLM` for an implementation that calls a local model
> (e.g. Ollama) — since both implement interfaces (`domain.Embedder`,
> `domain.LLMClient`), just create `internal/infrastructure/ollama/` and swap the
> line in the composition root. The rest of the pipeline (S3, SQS, DynamoDB, Textract)
> works normally on LocalStack.

### Step by step — running locally

#### 1. Clone and configure

```bash
git clone https://github.com/danubiobwm/chatbot-rag-go.git
cd chatbot-rag-go
cp .env.example .env
# edit .env if you want to use real Slack/WhatsApp tokens
export $(cat .env | xargs)
```

#### 2. Start local infrastructure

```bash
make up
```

This starts LocalStack (S3, SQS with DLQ, DynamoDB, Textract mock) and OpenSearch.
The `deploy/localstack-init/init.sh` script runs automatically and creates:

- bucket `chatbot-regulatorio-docs`
- queue `extraction-queue` with redrive policy to `extraction-queue-dlq` (3 retries)
- tables `ChatbotDocuments` (with GSI by hash), `ChatbotChunks`, `ChatbotSessions` (with TTL)

Verify it's up:

```bash
curl http://localhost:4566/_localstack/health
curl http://localhost:9200
```

#### 3. Create the vector index in OpenSearch

```bash
curl -X PUT "http://localhost:9200/chatbot-regulatorio" -H "Content-Type: application/json" -d '{
  "settings": { "index": { "knn": true } },
  "mappings": {
    "properties": {
      "document_id": { "type": "keyword" },
      "content": { "type": "text" },
      "embedding": { "type": "knn_vector", "dimension": 1024 }
    }
  }
}'
```

#### 4. Install Go dependencies

```bash
go mod tidy
```

#### 5. Run tests

```bash
make test
```

#### 6. Run the API and worker

In two terminals:

```bash
make run-api      # starts the webhook on :8080
make run-worker   # starts consuming the extraction queue
```

#### 7. Simulate a question

```bash
curl -X POST http://localhost:8080/webhook/message \
  -H "Content-Type: application/json" \
  -d '{"channel":"slack","user_id":"U123","text":"What is the home office policy?"}'
```

If real Bedrock is configured (`AWS_REGION` + valid credentials, without
`AWS_ENDPOINT_URL` for that specific call), the response will be generated based
on context retrieved from OpenSearch.

### Environment variables

See `.env.example`. The main ones:

| Variable | Description |
|---|---|
| `AWS_ENDPOINT_URL` | If set, all AWS clients point to LocalStack. Leave empty to use real AWS |
| `BEDROCK_LLM_MODEL_ID` | Which Bedrock model to use for response generation |
| `OPENSEARCH_URL` | OpenSearch endpoint (local or AWS managed domain) |
| `SLACK_BOT_TOKEN` / `WHATSAPP_TOKEN` | Output channel credentials |

### Tests

```bash
make test   # unit tests, with -race and coverage
make lint   # go vet + staticcheck
```

Usecases are tested with mocks of `domain` interfaces (`test/mocks`),
with no network or AWS dependencies — that's why investing in small interfaces
(ISP) from the start was worth it.

### Technical decisions and trade-offs

- **DLQ on all queues**: in the v1 of the project (based only on diagrams), a Textract
  or LLM segmentation failure would simply lose the document. Here, after N retries,
  the message falls into the DLQ and can be inspected/reprocessed.
- **Deterministic segmentation fallback**: we never let an LLM failure block the entire
  pipeline — see `LLMSegmenter` (decorator).
- **Hash-based deduplication**: re-sending the same PDF does not reprocess or re-embed —
  see `IngestDocumentUseCase.Execute` + GSI `content_hash-index`.
- **In-memory response cache**: sufficient for portfolio/demo; in production with
  multiple instances, swap for Redis/ElastiCache — only the `domain.ResponseCache`
  implementation changes.
- **Native DynamoDB TTL on sessions**: avoids manual cleanup jobs.

### Roadmap

- [ ] Telegram channel adapter (demonstrating Open/Closed in practice)
- [ ] Prometheus metrics + OpenTelemetry tracing (replacing the simple `Notifier`)
- [ ] `infrastructure/ollama` implementation to run without Bedrock costs
- [ ] IaC (Terraform) for the real AWS environment, mirroring the `docker-compose`
