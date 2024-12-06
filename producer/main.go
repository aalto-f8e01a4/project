package main

import (
	"bufio"
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	shared "trading-app/shared"

	"github.com/IBM/sarama"
	"google.golang.org/protobuf/proto"
)

func main() {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Flush.MaxMessages = 10_000
	config.Producer.MaxMessageBytes = 10_000_000

	producer, err := sarama.NewAsyncProducer([]string{"kafka.kafka.svc.cluster.local:9092"}, config)

	if err != nil {
		panic(err)
	}

	defer func() {
		if err := producer.Close(); err != nil {
			panic(err)
		}
	}()

	topic := "trades"

	files, err := filepath.Glob("../data/debs2022-*-10-11-*.csv") //filepath.Glob("../data/debs2022-*.csv")
	if err != nil {
		panic(err)
	}

	done := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-producer.Successes():
				// ignored
			case err := <-producer.Errors():
				log.Printf("Error producing message: %v\n", err)
			case <-done:
				return
			}
		}
	}()

	realStartTime := time.Now()
	fakeTargetTime := time.Date(2021, 11, 10, 10, realStartTime.Minute(), realStartTime.Second(), realStartTime.Nanosecond(), time.UTC)
	timeOffset := fakeTargetTime.Sub(realStartTime)

	log.Printf("Time offset: %v ms\n", timeOffset.Milliseconds())

	for _, file := range files {
		log.Printf("Processing file: %s\n", file)
		f, err := os.Open(file)
		if err != nil {
			log.Printf("Error opening file %s: %v\n", file, err)
			continue
		}
		defer f.Close()

		reader := csv.NewReader(bufio.NewReaderSize(f, 64*1024))
		reader.Comment = '#'
		reader.FieldsPerRecord = -1

		_, err = reader.Read()
		if err != nil {
			log.Printf("Error reading header from file %s: %v\n", file, err)
			continue
		}

		const (
			idIdx          = 0
			secTypeIdx     = 1
			lastIdx        = 21
			tradingTimeIdx = 23
			tradingDateIdx = 26
		)

		i := 0

		for {
			i++
			if i%100_000 == 0 {
				log.Printf("Processed %d records\n", i)
			}

			record, err := reader.Read()
			if err != nil {
				break
			}

			id, secType, last, tradingTime, tradingDate :=
				record[idIdx], record[secTypeIdx], record[lastIdx], record[tradingTimeIdx], record[tradingDateIdx]

			lastValue, err := strconv.ParseFloat(last, 64)
			if err != nil {
				continue
			}

			if tradingTime == "" || tradingDate == "" {
				continue
			}

			if secType != "E" && secType != "I" {
				log.Printf("Skipping record %s, unknown secType: %s\n", id, secType)
				continue
			}

			day, month, year := tradingDate[0:2], tradingDate[3:5], tradingDate[6:]

			composedTradingTime := year + "-" + month + "-" + day + " " + tradingTime

			currentFakeTime := time.Now().Add(timeOffset)
			parsedTradingTime, err := time.Parse("2006-01-02 15:04:05.000", composedTradingTime)

			if err != nil {
				log.Printf("Error parsing trading time: %v\n", err)
				continue
			}

			if parsedTradingTime.Before(fakeTargetTime) {
				continue
			}

			if parsedTradingTime.After(currentFakeTime) {
				sleepTime := parsedTradingTime.Sub(currentFakeTime)
				//log.Printf("Sleeping for %v\n", sleepTime)
				time.Sleep(sleepTime)
			}

			tradeMessage := &shared.Trade{
				Id:          id,
				SecType:     secType,
				Last:        lastValue,
				TradingTime: composedTradingTime,
			}
			messageContent, err := proto.Marshal(tradeMessage)
			if err != nil {
				log.Printf("Error marshalling trade message: %v\n", err)
				continue
			}

			message := &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(messageContent),
			}

			producer.Input() <- message
		}
	}

	log.Printf("All done!\n")

	producer.AsyncClose()
	done <- true
}
