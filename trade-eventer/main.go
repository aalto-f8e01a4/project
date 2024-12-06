package main

import (
	"context"
	"log"
	"time"
	shared "trading-app/shared"

	"database/sql"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/IBM/sarama"
	"google.golang.org/protobuf/proto"
)

const (
	EMA_38_ALPHA  = 2.0 / (1.0 + 38.0)
	EMA_100_ALPHA = 2.0 / (1.0 + 100.0)
)

type SecurityState struct {
	ema38           float64
	ema100          float64
	last            float64
	lastTradingTime string
	hasNew          bool
	ema38Above      bool
}

type Consumer struct {
	ready      chan bool
	securities map[string]*SecurityState
	producer   sarama.AsyncProducer
}

func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	close(c.ready)
	return nil
}

func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	c.ready = make(chan bool)
	return nil
}

func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg := <-claim.Messages():
			if msg == nil {
				return nil
			}

			trade := &shared.Trade{}
			if err := proto.Unmarshal(msg.Value, trade); err != nil {
				log.Printf("Error unmarshalling trade: %v\n", err)
				continue
			}

			state, exists := c.securities[trade.Id]
			if !exists {
				state = &SecurityState{}
				c.securities[trade.Id] = state
				state.ema38 = 0
				state.ema100 = 0
				state.ema38Above = false
			}

			state.last = trade.Last
			state.lastTradingTime = trade.TradingTime
			state.hasNew = true

			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func (c *Consumer) UpdateSecurityStates() {
	for id, s := range c.securities {
		if s.hasNew {
			newEma38 := (s.last)*EMA_38_ALPHA + s.ema38*(1-EMA_38_ALPHA)
			newEma100 := (s.last)*EMA_100_ALPHA + s.ema100*(1-EMA_100_ALPHA)

			oldEma38Above := s.ema38Above
			newEma38Above := newEma38 > newEma100

			if s.ema38 != 0 && s.ema100 != 0 && oldEma38Above != newEma38Above {
				actionType := shared.ActionType_ACTION_TYPE_BUY
				if !newEma38Above {
					actionType = shared.ActionType_ACTION_TYPE_SELL
				}

				action := &shared.Action{
					Id:          id,
					Type:        actionType,
					TradingTime: s.lastTradingTime,
				}

				actionBytes, err := proto.Marshal(action)
				if err != nil {
					log.Printf("Error marshalling action: %v\n", err)
					continue
				}

				c.producer.Input() <- &sarama.ProducerMessage{
					Topic: "actions",
					Value: sarama.ByteEncoder(actionBytes),
				}

				log.Printf("Generated %s signal for %s (EMA38: %.4f, EMA100: %.4f)\n",
					action.Type, action.Id, newEma38, newEma100)
			} else {
				log.Printf("Security ID %s did not change, (EMA38: %.4f, EMA100: %.4f, oldAbove: %t)\n",
					id, newEma38, newEma100, s.ema38Above)
			}

			s.ema38 = newEma38
			s.ema100 = newEma100
			s.ema38Above = newEma38Above
			s.hasNew = false
		} else {
			log.Printf("Security ID %s was not updated\n", id)
		}
	}
}

func main() {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Flush.MaxMessages = 100
	config.Producer.MaxMessageBytes = 100_000
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()

	brokers := []string{"kafka.kafka.svc.cluster.local:9092"}

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := producer.Close(); err != nil {
			panic(err)
		}
	}()

	consumerGroup, err := sarama.NewConsumerGroup(brokers, "trade-eventer", config)
	if err != nil {
		panic(err)
	}

	consumer := Consumer{
		ready:      make(chan bool),
		securities: make(map[string]*SecurityState),
		producer:   producer,
	}
	ctx := context.Background()

	db, err := sql.Open("clickhouse", "clickhouse://default:default@clickhouse.clickhouse.svc.cluster.local:9000/default")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query(`WITH
		2/(1+38) AS alpha_38,
		-ln(2)/ln(1-alpha_38) AS half_time_38,
		2/(1+100) AS alpha_100,
		-ln(2)/ln(1-alpha_100) AS half_time_100,
		trading_data AS (
			SELECT
				toStartOfFiveMinutes(trading_time) AS window_start,
				last_value(last) AS value,
				row_number() OVER (PARTITION BY id ORDER BY window_start ASC) AS t,
				id
			FROM trades
			WHERE NOT (last = 0 AND toHour(trading_time) = 2 AND toMinute(trading_time) = 0 AND toSecond(trading_time) = 0)
			GROUP BY window_start, id
			ORDER BY window_start ASC
		)
	SELECT
		id,
		exponentialMovingAverage(half_time_38)(value, t) AS ema_38,
		exponentialMovingAverage(half_time_100)(value, t) AS ema_100
	FROM trading_data 
	GROUP BY id`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var ema38, ema100 float64
		if err := rows.Scan(&id, &ema38, &ema100); err != nil {
			log.Printf("Error scanning row: %v\n", err)
			continue
		}
		consumer.securities[id] = &SecurityState{
			ema38:      ema38,
			ema100:     ema100,
			ema38Above: ema38 > ema100,
		}
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}

	log.Printf("Loaded %d EMA values\n", len(consumer.securities))

	go func() {
		for {
			now := time.Now()
			next := now.Truncate(5 * time.Minute).Add(5 * time.Minute)
			time.Sleep(time.Until(next))
			log.Println("Updating security states")
			consumer.UpdateSecurityStates()
		}
	}()

	for {
		err := consumerGroup.Consume(ctx, []string{"trades"}, &consumer)
		if err != nil {
			log.Printf("Error from consumer: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		consumer.ready = make(chan bool)
	}
}
