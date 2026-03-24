# Distributed Tracing - CEP Weather Service

Sistema distribuído em Go com rastreamento distribuído via OpenTelemetry e Zipkin.

## Arquitetura

```
Client → Service A (porta 8080) → Service B (porta 8081)
                                        ↓
                                   ViaCEP API
                                   WeatherAPI
         ↑                              ↑
    OTEL Collector (porta 4317/4318)
         ↓
      Zipkin (porta 9411)
```

## Pré-requisitos

- Docker e Docker Compose instalados
- Chave de API do [WeatherAPI](https://www.weatherapi.com/) (gratuita)
- Go 1.22+ (para desenvolvimento local)

## Configuração

1. Clone o repositório e acesse o diretório:

```bash
cd distributed-tracing
```

2. Copie o arquivo de variáveis de ambiente e configure sua API key:

```bash
cp .env.example .env
# Edite .env e insira sua WEATHER_API_KEY
```

3. Inicialize os módulos Go (necessário na primeira vez):

```bash
cd service-a && go mod tidy && cd ..
cd service-b && go mod tidy && cd ..
```

## Executando

```bash
docker-compose up --build
```

Aguarde todos os serviços subirem (pode levar alguns segundos para o OTEL Collector se conectar ao Zipkin).

## Realizando uma Requisição

Envie um POST para o Service A na porta 8080:

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"cep": "01310100"}'
```

### Resposta de Sucesso (200)

```json
{
  "city": "São Paulo",
  "temp_C": 25.0,
  "temp_F": 77.0,
  "temp_K": 298.0
}
```

### Respostas de Erro

| Situação | Status | Mensagem |
|---|---|---|
| CEP com formato inválido | 422 | `invalid zipcode` |
| CEP não encontrado | 404 | `can not find zipcode` |

## Visualizando os Traços no Zipkin

1. Acesse [http://localhost:9411](http://localhost:9411)
2. Clique em **"Run Query"** para listar os traços recentes
3. Clique em um traço para ver o fluxo completo: `service-a → service-b`
4. Expanda os spans para ver os detalhes de:
   - `fetch-cep-viacep` — tempo de resposta da API ViaCEP
   - `fetch-temperature-weatherapi` — tempo de resposta da WeatherAPI

## Estrutura do Projeto

```
distributed-tracing/
├── service-a/                  # Serviço de entrada (validação de CEP)
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── service-b/                  # Serviço de orquestração (clima)
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── otel-collector-config.yaml  # Configuração do OTEL Collector
├── docker-compose.yaml
├── .env.example
└── README.md
```

## Variáveis de Ambiente

### Service A
| Variável | Padrão | Descrição |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `otel-collector:4317` | Endpoint do OTEL Collector |
| `SERVICE_B_URL` | `http://service-b:8081` | URL do Service B |

### Service B
| Variável | Padrão | Descrição |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `otel-collector:4317` | Endpoint do OTEL Collector |
| `WEATHER_API_KEY` | — | Chave da WeatherAPI (obrigatória) |
