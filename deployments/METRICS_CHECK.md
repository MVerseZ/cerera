# Проверка метрик Cerera

## Быстрая проверка

### 1. Проверка метрик напрямую

После перезапуска ноды, проверьте эндпоинт `/metrics`:

```bash
# Для одной ноды
curl http://localhost:1337/metrics | grep -E "(miner_|pool_|consensus_|p2p_|chain_|rpc_)"

# Для нескольких нод
curl http://localhost:1338/metrics | grep -E "(miner_|pool_|consensus_|p2p_|chain_|rpc_)"
```

### 2. Проверка в Prometheus

1. Откройте http://localhost:9090
2. Перейдите в Status → Targets
3. Убедитесь, что все ноды в состоянии "UP"
4. В поиске введите название метрики, например:
   - `miner_blocks_mined_total`
   - `pool_size`
   - `consensus_voters_total`
   - `p2p_peers_connected`
   - `chain_height`
   - `rpc_requests_total`

### 3. Проверка в Grafana

1. Откройте http://localhost:3100
2. Войдите (admin/admin)
3. Перейдите в Dashboards → Browse
4. Выберите дашборд "Cerera Overview"
5. Убедитесь, что панели отображают данные

## Список всех новых метрик

### Miner метрики
- `miner_blocks_mined_total` - счетчик добытых блоков
- `miner_mining_attempts_total` - счетчик попыток майнинга
- `miner_mining_errors_total` - счетчик ошибок майнинга
- `miner_mining_duration_seconds` - гистограмма длительности майнинга
- `miner_pending_txs_in_block` - gauge pending транзакций
- `miner_status` - gauge статуса (0=stopped, 1=active)

### Pool метрики
- `pool_size` - gauge размера пула
- `pool_bytes` - gauge размера в байтах
- `pool_tx_added_total` - счетчик добавленных транзакций
- `pool_tx_removed_total` - счетчик удаленных транзакций
- `pool_tx_rejected_total` - счетчик отклоненных транзакций
- `pool_max_size` - gauge максимального размера

### Consensus метрики
- `consensus_voters_total` - gauge количества голосующих
- `consensus_nodes_total` - gauge количества нод
- `consensus_state` - gauge состояния (0=Follower, 1=Candidate, 2=Leader, 3=Validator, 4=Miner)
- `consensus_nonce` - gauge текущего nonce
- `consensus_period_advances_total` - счетчик переходов периодов
- `consensus_status` - gauge статуса

### P2P метрики (дополнительные)
- `p2p_peers_connected` - gauge подключенных пиров
- `p2p_peer_disconnects_total` - счетчик отключений
- `p2p_stream_errors_total` - счетчик ошибок потоков

### Chain метрики (дополнительные)
- `chain_height` - gauge высоты цепочки
- `chain_block_size_bytes` - гистограмма размера блоков
- `chain_block_gas_used` - гистограмма использованного газа

### RPC метрики
- `rpc_requests_total{method="..."}` - счетчик запросов по методам
- `rpc_requests_duration_seconds{method="..."}` - гистограмма длительности
- `rpc_requests_errors_total{method="..."}` - счетчик ошибок по методам

## Если метрики не видны

1. **Перезапустите ноды**:
   ```bash
   docker-compose -f docker-compose-full.yml restart
   ```

2. **Проверьте логи нод**:
   ```bash
   docker-compose -f docker-compose-full.yml logs node1 | grep -i metric
   ```

3. **Проверьте, что Prometheus собирает метрики**:
   - Откройте http://localhost:9090
   - Status → Targets → проверьте статус всех нод

4. **Обновите дашборд Grafana**:
   - Откройте дашборд
   - Нажмите кнопку обновления (↻) или F5

5. **Проверьте временной диапазон в Grafana**:
   - Убедитесь, что выбран правильный временной диапазон (например, "Last 5 minutes")

## Тестирование метрик

Для проверки, что метрики обновляются:

1. **Miner метрики**: Запустите майнинг - должны обновляться `miner_blocks_mined_total`, `miner_mining_attempts_total`
2. **Pool метрики**: Отправьте транзакцию - должны обновляться `pool_tx_added_total`, `pool_size`
3. **Chain метрики**: При добавлении блока - должны обновляться `chain_height`, `chain_blocks_total`
4. **RPC метрики**: Сделайте RPC запрос - должны обновляться `rpc_requests_total`
