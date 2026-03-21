# Ponderada M09 - Fila

A atividade ponderada contempla um backend de coleta e processamento de dados de sensores industriais, desenvolvido como solução para o desafio de escalabilidade e confiabilidade no recebimento de telemetria de dispositivos embarcados, dado o contexto fornecido para a atividade.


## Problema

Uma empresa de monitoramento industrial opera dispositivos embarcados que coletam leituras periódicas de sensores distribuídos em diferentes ambientes (temperatura, umidade, vibração, luminosidade, presença). Com o crescimento de dispositivos, o backend passa a enfrentar:

- **Alto volume de requisições simultâneas**, com risco de perda de dados em picos
- **Gargalo no processamento síncrono**, onde persistir no banco diretamente no endpoint compromete a latência e a estabilidade
- **Necessidade de resiliência**, garantindo que falhas temporárias no banco não causem perda de leituras

A solução propõe uma arquitetura de mensageria para asborver esses picos e garantir que nenhum dado seja perdido.

---

## Decisões realizadas para a realização da atividade

### 1. Go + Gin para o Backend

Go foi escolhido pela sua eficiência no em lidar com centenas de requisições simultâneas com baixo consumo de memória. O Gin adiciona roteamento e binding de JSON com validação declarativa via struct tags, reduzindo o código boilerplate. A decisão de utilizar também foi feita visando a aplicação daquilo que estamos aprendendo em sala de aula com as instruções.

---

### 2. Arquitetura Producer/Consumer com RabbitMQ

O processamento síncrono direto no endpoint cria acoplamento entre a velocidade de resposta da API e a velocidade do banco de dados. Em picos de carga, isso causa aumento de latência e eventual rejeição de requisições. Dessa forma, o backend responde em sub-milissegundo, independente do estado do banco, o banco recebe escritas em ritmo controlado pelo consumer, e em caso de falhas temporárias no PostgreSQL, não ocorre perda de dados.

```
[Dispositivo IoT]
      │ HTTP POST /telemetry
      ▼
[Backend Go/Gin]  ←── responde 200 em ~1ms
      │ Publish
      ▼
[RabbitMQ — telemetry_queue]
      │ Consume
      ▼
[Middleware Go]
      │ INSERT
      ▼
[PostgreSQL]
```

---

### 3. RabbitMQ

Com o RabbitMQ, a fila foi configurada como durável para sobreviver a reinicializações do broker. O consumer garante que nenhuma leitura seja descartada de maneira silenciosa.

---

### 4. PostgreSQL 

O modelo de dados foi projetado para suportar qualquer tipo de sensor sem alterações de schema. O campo `value` é `DOUBLE PRECISION`, cobrindo tanto leituras analógicas (valores contínuos) quanto discretas (0.0 e 1.0). Índices foram criados nas colunas de maior seletividade para consultas analíticas:

```sql
CREATE INDEX idx_telemetry_device_id   ON telemetry_readings (device_id);
CREATE INDEX idx_telemetry_sensor_type ON telemetry_readings (sensor_type);
CREATE INDEX idx_telemetry_timestamp   ON telemetry_readings (timestamp DESC);
CREATE INDEX idx_telemetry_device_time ON telemetry_readings (device_id, timestamp DESC);
```

---

### 5. Infraestrutura Conteinerizada

A conteinerização garante reprodutibilidade total do ambiente. Os healthchecks evitam race conditions na inicialização: o backend só sobe após RabbitMQ e PostgreSQL estarem prontos, e o middleware aguarda o mesmo. 

---

### 6. k6 via Docker para Testes de Carga

Realizar os testes de carga k6 no Docker elimina dependências de ambiente e garante que qualquer pessoa com Docker consiga reproduzir os testes com o comando. O uso de `profiles` impede que os containers de teste subam junto com a stack principal (`docker compose up`), sendo invocados explicitamente apenas quando necessário. Foram implementados 6 tipos de teste cobrindo diferentes cenários de carga.

---

## Estrutura

```
.
├── main.go                        # Entrypoint do backend
├── dockerfile                     # Build do backend
├── docker-compose.yml             # Orquestração de todos os serviços
├── go.mod / go.sum
│
├── handlers/
│   ├── telemetry_handler.go       # Handler POST /telemetry
│   └── telemetry_handler_test.go
│
├── models/
│   └── telemetry.go               # Struct e tipos (analog/discrete)
│
├── rabbitmq/
│   ├── client.go                  # Conexão e publish no RabbitMQ
│   └── interface.go               # Interface Publisher (testabilidade)
│
├── database/
│   ├── client.go                  # Conexão com PostgreSQL
│   └── interface.go               # Interface TelemetryRepository
│
├── migrations/
│   └── init.sql                   # Schema e índices do banco
│
├── middleware/
│   ├── main.go                    # Consumer RabbitMQ → PostgreSQL
│   ├── main_test.go
│   ├── dockerfile
│   ├── go.mod / go.sum
│
└── k6/
    ├── lib/
    │   └── helpers.js             # Payloads e utilitários compartilhados
    ├── tests/
    │   ├── smoke_test.js          # 5 VUs / 30s
    │   ├── validation_test.js     # Regras de negócio
    │   ├── load_test.js           # 50 VUs / ~4,5min
    │   ├── stress_test.js         # Até 200 VUs / ~6,5min
    │   ├── spike_test.js          # Burst 300 VUs / ~3,5min
    │   └── soak_test.js           # 30 VUs / 10min
    └── README.md                  # Instruções dos testes
```

---

## Como Executar

### Pré-requisitos

- Docker
- Docker Compose v2+

### Subir a stack

```bash
docker compose up -d
```

Aguarde todos os serviços ficarem healthy (~30–60s):

```bash
docker compose ps
```

A API estará disponível em `http://localhost:8080`.

### Enviar uma leitura de telemetria

```bash
curl -X POST http://localhost:8080/telemetry \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device-0001",
    "timestamp": "2026-03-20T10:00:00Z",
    "sensor_type": "temperature",
    "value_type": "analog",
    "value": 42.7
  }'
```

Resposta esperada:
```json
{"message": "telemetry queued successfully"}
```

### Parar a stack

```bash
docker compose down
```

Para remover também os volumes (dados do banco):

```bash
docker compose down -v
```

---

## Testes de Carga

Os testes rodam via Docker, sem instalação do k6. A stack principal deve estar rodando antes de executar qualquer teste.

```bash
# Validação básica (30s)
docker compose run --rm k6-smoke

# Regras de negócio (1min)
docker compose run --rm k6-validation

# Carga sustentada — 50 VUs (~4,5min)
docker compose run --rm k6-load

# Ponto de ruptura — até 200 VUs (~6,5min)
docker compose run --rm k6-stress

# Burst súbito — 300 VUs em 5s (~3,5min)
docker compose run --rm k6-spike

# Resistência prolongada — 30 VUs por 10min
docker compose run --rm k6-soak
```

Cada teste imprime um sumário formatado no terminal com throughput, latência (p50/p90/p95/p99/max), taxa de erro e resultado dos thresholds.

---

## Relatório de Resultados

Os resultados completos com análise interpretativa, identificação de gargalos e melhorias recomendadas estão documentados em:

📄 [`docs/relatorio-testes-carga.md`](docs/relatorio-testes-carga.md)

### Resumo dos resultados

| Teste | Throughput | p95 | Taxa de Erro | Resultado |
|---|---|---|---|---|
| Smoke | 9,95 req/s | 2,0ms | 0% | ✅ |
| Validation | — | — | 0% (5xx) | ✅ |
| Load | 118,20 req/s | 1,3ms | 0% | ✅ |
| Stress | 976,28 req/s | 2,0ms | 0% | ✅ |
| Spike | ~7.400 req/s | 26,8ms | 0% | ✅ |
| Soak | 35,96 req/s | 1,3ms | 0% | ✅ |

O sistema demonstrou alta resiliência em todos os cenários.