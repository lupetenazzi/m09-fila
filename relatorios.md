# Relatório de Testes de Carga 

Este relatório documenta os resultados dos testes de carga. Os testes foram executados com a ferramenta **k6** via Docker, sem instalação local, avaliando o comportamento da aplicação sob diferentes níveis de estresse.

O objetivo central foi mensurar **throughput**, **latência**, **taxa de erro** e **capacidade de enfileiramento** da plataforma, identificando possíveis gargalos e limites operacionais.

---

## 2. Arquitetura Testada

```
[k6 (gerador de carga)]
         │
         ▼ HTTP POST /telemetry
[Backend Go + Gin :8080]
         │
         ▼ Publish
[RabbitMQ :5672 — fila: telemetry_queue]
         │
         ▼ Consume
[Middleware Go — consumer]
         │
         ▼ INSERT
[PostgreSQL :5432 — tabela: telemetry_readings]
```

O backend atua exclusivamente como **produtor**, ele recebe a requisição HTTP, valida o payload e publica na fila. O middleware consome de forma assíncrona e persiste no banco. 

---

## 3. Metodologia

Foram executados 6 tipos de teste, cada um com objetivo distinto:

| Teste | VUs | Duração | Objetivo |
|---|---|---|---|
| Smoke | 5 | 30s | Validação de sanidade (API responde corretamente?) |
| Validation | 3 | 1min | Regras de negócio (aceita válidos, rejeita inválidos) |
| Load | 0 → 50 → 0 | ~4,5min | Comportamento sob carga típica de produção |
| Stress | 0 → 200 → 0 | ~6,5min | Ponto de ruptura e degradação progressiva |
| Spike | 10 → 300 → 10 | ~3,5min | Burst súbito (capacidade de enfileiramento) |
| Soak | 0 → 30 → 0 | ~10min | Resistência prolongada (memory leak e degradação) |

Os payloads simulam uma frota de 50 dispositivos IoT enviando leituras analógicas (temperatura, pressão, rpm, etc.) e discretas (0 ou 1), com distribuição de 80% analógico e 20% discreto.

---

## 4. Resultados

### 4.1 Smoke Test

| Métrica | Valor |
|---|---|
| Requisições totais | 300 |
| Throughput | 9,95 req/s |
| Taxa de erro | 0% |
| p50 | ~0,5ms |
| p95 | 2,0ms |
| max | 2,7ms |

**Resultado: ✅ PASSOU**

A API respondeu corretamente a todas as requisições com latência sub-milissegundo na mediana. O teste confirmou que a stack estava operacional antes da aplicação de cargas maiores.

---

### 4.2 Validation Test

| Métrica | Valor |
|---|---|
| Iterações completas | 222 |
| Válidos aceitos (200) | 222 |
| Inválidos rejeitados (400) | 444 |
| Erros inesperados (5xx) | 0% |

**Resultado: ✅ PASSOU**

Todos os cenários de validação funcionaram conforme esperado:
- Payloads analógicos e discretos válidos foram aceitos com HTTP 200
- Campos obrigatórios ausentes retornaram HTTP 400
- `value_type: discrete` com `value: 5` (inválido) retornou HTTP 400
- JSON malformado e body vazio retornaram HTTP 400
- Nenhum erro interno (5xx) foi gerado por payload inválido

---

### 4.3 Load Test

| Métrica | Valor |
|---|---|
| Requisições totais | 31.970 |
| Throughput | 118,20 req/s |
| Sucessos | 31.970 |
| Taxa de erro | 0% |
| p50 | ~0,6ms |
| p90 | 1,1ms |
| p95 | 1,3ms |
| p99 | ~2,5ms |
| max | 9,1ms |

**Resultado: ✅ PASSOU** (todos os thresholds)

Com 50 VUs simultâneos durante 3 minutos de carga sustentada, a aplicação manteve **0% de erro** e latência consistentemente abaixo de 10ms. O throughput de 118 req/s com p95 de 1,3ms demonstra ampla folga em relação aos thresholds definidos (p95 < 800ms, p99 < 1500ms).

---

### 4.4 Stress Test

| Métrica | Valor |
|---|---|
| Requisições totais | 380.822 |
| Throughput médio | 976,28 req/s |
| Sucessos | 380.822 |
| Taxa de erro | 0% |
| p90 | 1,4ms |
| p95 | 2,0ms |
| max | 38,8ms |

**Resultado: ✅ PASSOU**

A aplicação não apresentou ponto de ruptura até 200 VUs simultâneos. O throughput escalou de ~120 req/s (30 VUs) até ~976 req/s (200 VUs) sem nenhuma falha. A latência máxima de 38,8ms ocorreu provavelmente durante o escalonamento mais agressivo (100 → 200 VUs em 30s), mas a mediana e o p95 permaneceram estáveis. O threshold de erro < 10% não foi sequer aproximado.

---

### 4.5 Spike Test

| Métrica | Valor |
|---|---|
| Total enviado | 1.337.740 |
| Enfileirado com êxito | 1.337.740 |
| Taxa de enfileiramento | 100% |
| Taxa de erro | 0% |
| p95 | 26,8ms |
| max | 154,7ms |

**Resultado: ✅ PASSOU**

O resultado mais expressivo da bateria. Com um burst de **10 → 300 VUs em apenas 5 segundos**, a plataforma enfileirou **1,3 milhão de mensagens com 100% de sucesso**. A latência máxima de 154,7ms ocorreu no pico do burst e se normalizou após o ramp-down. O RabbitMQ absorveu a carga sem rejeitar nenhuma mensagem, validando o design de desacoplamento da arquitetura.

---

### 4.6 Soak Test

| Métrica | Valor |
|---|---|
| Requisições totais | 21.577 |
| Throughput | 35,96 req/s |
| Taxa de erro | 0% |
| Erros de pool de conexão | 0 |
| p90 | 1,1ms |
| p95 | 1,3ms |
| max | 7,8ms |

**Resultado: ✅ PASSOU**

Em 10 minutos de carga contínua com 30 VUs, não foram detectados sinais de degradação progressiva, vazamento de memória ou esgotamento de pool de conexões. A latência p95 de 1,3ms ao final do teste é idêntica à do início, confirmando estabilidade temporal da plataforma.

---

## 5. Análise Interpretativa

### 5.1 Pontos Fortes

**Arquitetura de fila como principal diferencial.** O resultado do spike test (1,3 milhão de mensagens enfileiradas com 0% de perda) demonstra que a escolha do RabbitMQ como buffer entre o backend e o banco de dados é o fator determinante da resiliência da plataforma. O backend nunca ficou sobrecarregado porque seu trabalho se limita a validar o payload e publicar na fila, operações extremamente rápidas (sub-milissegundo).

**Latência consistentemente baixa.** Em todos os testes, o p95 ficou abaixo de 27ms mesmo no cenário de maior pressão (300 VUs em burst). Para uma aplicação IoT onde dispositivos enviam leituras periódicas, isso garante que nenhum dispositivo ficará esperando resposta por tempo significativo.

**Ausência total de erros sob carga.** Nenhum dos testes de carga (load, stress, spike, soak) produziu HTTP 5xx. Isso indica que nem o backend nem o RabbitMQ atingiram saturação dentro dos limites testados.

---

### 5.2 Gargalos Identificados

**O ponto de ruptura não foi encontrado.** O stress test chegou a 200 VUs e 976 req/s sem falhas. Isso significa que os limites reais da plataforma estão além do que foi testado. Para descobrir o ponto de ruptura seria necessário escalar para 500–1000 VUs ou testar em ambiente com recursos computacionais mais restritos.

**O middleware é o gargalo não testado.** Toda a bateria testou apenas o caminho de **entrada** (HTTP → fila). O middleware que consome a fila e persiste no PostgreSQL não foi avaliado. Em cenários de burst prolongado (diferente do spike de 3 minutos), a fila poderia acumular mensagens mais rápido do que o middleware consegue processar, causando crescimento ilimitado do backlog. Um teste específico de consumo seria necessário.

**PostgreSQL sob escrita massiva não foi avaliado.** O banco recebe os INSERTs de forma assíncrona pelo middleware. Sob carga prolongada com throughput de ~1000 req/s, o volume de escritas no PostgreSQL pode se tornar um gargalo. Os índices criados na tabela `telemetry_readings` ajudam nas leituras, mas podem impactar a velocidade de escrita em volume alto.

---

### 5.3 Melhorias Recomendadas

**1. Testar o ponto de ruptura real.**
Executar o stress test com 500–2000 VUs para encontrar onde a taxa de erro começa a subir. Isso fornece o número concreto de capacidade máxima da plataforma.

**2. Adicionar circuit breaker no backend.**
Em caso de falha do RabbitMQ, o backend retorna HTTP 500 imediatamente. Um circuit breaker evitaria que falhas em cascata derrubassem o serviço inteiro ao detectar indisponibilidade do broker.

---

## 6. Conclusão

O projeto demonstrouuma boa performance e resiliência em todos os cenarios testados. Com 976 req/s sustentados sem erros e 1,3 milhão de mensagens enfileiradas em um spike, o sistema está, teoricamente, pronto para suportar frotas IoT de escala considerável.

Os próximos passos prioritários são avaliar o comportamento do middleware sob carga sustentada e descobrir o ponto de ruptura real escalando o número de VUs além de 200.