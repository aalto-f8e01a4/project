-- trades
DROP TABLE IF EXISTS trades_kafka_raw;
CREATE TABLE trades_kafka_raw (
    id String,
    secType String,
    last Float64,
    tradingTime String
) ENGINE = Kafka SETTINGS
    kafka_broker_list = 'kafka.kafka.svc.cluster.local:9092',
    kafka_topic_list = 'trades',
    kafka_group_name = 'clickhouse',
    kafka_format = 'ProtobufSingle',
    kafka_schema = 'trade.proto:Trade',
    kafka_num_consumers = 1;

DROP TABLE IF EXISTS trades;
CREATE TABLE trades (
    id String,
    sec_type Enum('E', 'I'),
    last Float64,
    trading_time DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(trading_time)
ORDER BY id;

DROP VIEW IF EXISTS trades_mv;
CREATE MATERIALIZED VIEW trades_mv TO trades AS
SELECT
    id,
    secType,
    last,
    toDateTime64(tradingTime, 3, 'Etc/GMT+2') AS trading_time
FROM trades_kafka_raw;

-- actions
DROP TABLE IF EXISTS actions_kafka_raw;
CREATE TABLE actions_kafka_raw (
    id String,
    type Enum('BUY' = 0, 'SELL' = 1),
    tradingTime String
) ENGINE = Kafka SETTINGS
    kafka_broker_list = 'kafka.kafka.svc.cluster.local:9092',
    kafka_topic_list = 'actions',
    kafka_group_name = 'clickhouse',
    kafka_format = 'ProtobufSingle',
    kafka_schema = 'action.proto:Action',
    kafka_num_consumers = 1;

DROP TABLE IF EXISTS actions;
CREATE TABLE actions (
    id String,
    type Enum('BUY' = 0, 'SELL' = 1),
    trading_time DateTime64(3, 'UTC'),
    created_at DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (id, created_at);

DROP VIEW IF EXISTS actions_mv;
CREATE MATERIALIZED VIEW actions_mv TO actions AS
SELECT
    id,
    type,
    toDateTime64(tradingTime, 3, 'Etc/GMT+2') AS trading_time,
    _timestamp_ms AS created_at
FROM actions_kafka_raw;
