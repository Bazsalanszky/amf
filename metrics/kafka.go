// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"time"

	"github.com/omec-project/amf/factory"
	"github.com/omec-project/amf/logger"
	mi "github.com/omec-project/util/metricinfo"
	"github.com/segmentio/kafka-go"
)

type Writer struct {
	kafkaWriter *kafka.Writer
}

var StatWriter Writer

func InitialiseKafkaStream(config *factory.Configuration) error {
	if !*config.KafkaInfo.EnableKafka {
		logger.KafkaLog.Warnln("Kafka is disabled")
		return nil
	}

	brokerUrl := kafkaBrokerAddr(config.KafkaInfo.BrokerUri, config.KafkaInfo.BrokerPort)
	topicName := "sdcore-data-source-amf"

	logger.KafkaLog.Debugf("initialise kafka broker url: %s", brokerUrl)

	if config.KafkaInfo.Topic != "" {
		topicName = config.KafkaInfo.Topic
	}

	logger.KafkaLog.Debugf("initialise kafka Topic: %s", topicName)

	producer := kafka.Writer{
		Addr:                   kafka.TCP(brokerUrl),
		Topic:                  topicName,
		AllowAutoTopicCreation: true,
		// Hash routes same-key messages to the same partition, keeping this subscriber's Add before its
		// later Mod/Del events; keyless messages fall back to round robin.
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond,
	}

	StatWriter = Writer{
		kafkaWriter: &producer,
	}

	logger.KafkaLog.Debugf("initialising kafka stream with url[%s], topic[%s]", brokerUrl, topicName)
	return nil
}

// kafkaBrokerAddr returns the host:port of the Kafka broker: "kafka:9092" unless
// both brokerUri and brokerPort are configured. net.JoinHostPort brackets an
// IPv6 literal brokerUri, which fmt.Sprintf("%s:%d") did not: kafka-go read
// "fd00::1:9092" as a host name and the dial failed with "lookup fd00::1:9092".
// IPv4 addresses and hostnames give the same string as before. A brokerUri that
// is already bracketed is unwrapped first so it is not bracketed twice.
func kafkaBrokerAddr(brokerUri string, brokerPort int) string {
	if brokerUri == "" || brokerPort == 0 {
		return "kafka:9092"
	}
	if len(brokerUri) > 1 && brokerUri[0] == '[' && brokerUri[len(brokerUri)-1] == ']' {
		brokerUri = brokerUri[1 : len(brokerUri)-1]
	}
	return net.JoinHostPort(brokerUri, strconv.Itoa(brokerPort))
}

func GetWriter() Writer {
	return StatWriter
}

func (writer Writer) SendMessage(message []byte) error {
	return writer.sendMessage(kafka.Message{Value: message})
}

// SendMessageWithKey sends a message keyed for partition-stable, per-key ordering (see Hash balancer above).
func (writer Writer) SendMessageWithKey(key, message []byte) error {
	return writer.sendMessage(kafka.Message{Key: key, Value: message})
}

func (writer Writer) sendMessage(msg kafka.Message) error {
	if err := writer.kafkaWriter.WriteMessages(context.Background(), msg); err != nil {
		logger.KafkaLog.Errorf("kafka send message write error: %s", err.Error())
		return err
	}
	return nil
}

func (writer Writer) PublishUeCtxtEvent(ctxt mi.CoreSubscriber, op mi.SubscriberOp) error {
	smKafkaEvt := mi.MetricEvent{
		EventType:      mi.CSubscriberEvt,
		SubscriberData: mi.CoreSubscriberData{Subscriber: ctxt, Operation: op},
	}
	msg, err := json.Marshal(smKafkaEvt)
	if err != nil {
		logger.KafkaLog.Errorf("publishing ue context event error %s", err.Error())
		return err
	}
	logger.KafkaLog.Debugf("publishing ue context event: %s", string(msg))
	// Key by IMSI so this subscriber's events stay on one partition and keep their order.
	return StatWriter.SendMessageWithKey([]byte(ctxt.Imsi), msg)
}

func (writer Writer) PublishNfStatusEvent(msgEvent mi.MetricEvent) error {
	if msg, err := json.Marshal(msgEvent); err != nil {
		logger.KafkaLog.Errorf("publishing nf status marshal error: %s", err.Error())
		return err
	} else {
		logger.KafkaLog.Debugf("publishing nf status event: %s", string(msg))
		if err := StatWriter.SendMessage(msg); err != nil {
			logger.KafkaLog.Errorf("publishing nf status event error: %s", err.Error())
		}
	}
	return nil
}
