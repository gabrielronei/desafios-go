# Stress Test CLI

Ferramenta de linha de comando em Go para realizar testes de carga em serviços web.

## Parâmetros

| Flag | Descrição | Obrigatório |
|---|---|---|
| `--url` | URL do serviço a ser testado | Sim |
| `--requests` | Número total de requests | Sim |
| `--concurrency` | Número de chamadas simultâneas | Sim |

## Execução

### Via Docker (recomendado)

```bash
docker run stress-test --url=http://exemplo.com --requests=1000 --concurrency=10
```

### Build local

```bash
docker build -t stress-test .
```

### Sem Docker

```bash
go run main.go --url=http://exemplo.com --requests=1000 --concurrency=10
```

## Relatório

Ao final da execução é exibido um relatório com:

- URL testada e nível de concorrência
- Tempo total gasto na execução
- Total de requests realizados
- Quantidade de requests com status HTTP 200
- Distribuição dos demais códigos HTTP (404, 500, etc.)

### Exemplo de saída

```
========================================
         RELATÓRIO DE STRESS TEST
========================================
URL:                    https://exemplo.com
Concorrência:           10
Tempo total:            2.345s
Total de requests:      100
Requests com HTTP 200:  95
----------------------------------------
Distribuição de status HTTP:
  HTTP 200: 95
  HTTP 500: 3
  HTTP 404: 2
========================================
```
