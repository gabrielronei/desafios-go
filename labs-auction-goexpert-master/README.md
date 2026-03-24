# Auction Go Expert - Fechamento Automático de Leilões

Sistema de leilões em Go com fechamento automático via Goroutine.

## Funcionalidade Implementada

Ao criar um leilão, uma Goroutine é iniciada em background. Após o tempo configurado em `AUCTION_DURATION`, o status do leilão é alterado automaticamente para `Completed` (fechado) no MongoDB, sem intervenção manual.

## Pré-requisitos

- Docker e Docker Compose

## Executando

```bash
docker-compose up --build
```

A API estará disponível em `http://localhost:8080`.

## Variáveis de Ambiente

Configuradas em `cmd/auction/.env`:

| Variável | Padrão | Descrição |
|---|---|---|
| `AUCTION_DURATION` | `20s` | Tempo até o leilão ser fechado automaticamente. Aceita qualquer duração válida em Go: `20s`, `5m`, `1h`. |
| `AUCTION_INTERVAL` | `20s` | Intervalo usado pela validação de lances para checar expiração. |
| `BATCH_INSERT_INTERVAL` | `20s` | Intervalo máximo para inserção de lotes de lances. |
| `MAX_BATCH_SIZE` | `4` | Tamanho máximo do lote de lances. |

## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/auction` | Cria um leilão |
| `GET` | `/auction` | Lista leilões |
| `GET` | `/auction/:auctionId` | Busca leilão por ID |
| `GET` | `/auction/winner/:auctionId` | Lance vencedor |
| `POST` | `/bid` | Cria um lance |
| `GET` | `/bid/:auctionId` | Lista lances de um leilão |
| `GET` | `/user/:userId` | Busca usuário por ID |

### Exemplo de criação de leilão

```bash
curl -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "Notebook",
    "category": "Electronics",
    "description": "Notebook em ótimo estado para venda",
    "condition": 1
  }'
```

## Rodando os Testes

O teste de fechamento automático requer MongoDB. Suba o container antes:

```bash
docker-compose up -d mongodb

go test ./internal/infra/database/auction/... -v -run TestAuctionAutoClose -timeout 30s
```

O teste cria um leilão com duração de 2 segundos, aguarda 3 segundos e verifica que o status mudou para `Completed`.
