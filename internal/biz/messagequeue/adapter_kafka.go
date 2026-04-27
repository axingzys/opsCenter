package messagequeue

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaAdapter struct{}

func NewKafkaAdapter() *KafkaAdapter {
	return &KafkaAdapter{}
}

func (a *KafkaAdapter) Type() string {
	return MQTypeKafka
}

func (a *KafkaAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	conn, err := kafkaDial(ctx, instance)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	brokers, err := conn.Brokers()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Broker失败: %w", err)
	}
	version := "Kafka"
	if versions, err := conn.ApiVersions(); err == nil && len(versions) > 0 {
		version = fmt.Sprintf("Kafka API %d-%d", versions[0].MinVersion, versions[0].MaxVersion)
	}
	return &MQConnectionTestResult{
		Version:         version,
		Engine:          MQTypeKafka,
		BrokerCount:     len(brokers),
		ManagementReady: true,
		Message:         "Kafka Broker 连接成功",
	}, nil
}

func (a *KafkaAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	conn, err := kafkaDial(ctx, instance)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	now := time.Now()
	brokers, err := conn.Brokers()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Broker失败: %w", err)
	}
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("读取Kafka Topic元数据失败: %w", err)
	}

	snapshot := &MQMetadataSnapshot{
		Version:      "Kafka",
		Engine:       MQTypeKafka,
		HealthStatus: HealthStatusHealthy,
		Message:      "Kafka 元数据同步成功",
		SyncedAt:     now,
	}
	for _, broker := range brokers {
		name := fmt.Sprintf("broker-%d", broker.ID)
		snapshot.Brokers = append(snapshot.Brokers, &MQBroker{
			InstanceID: instance.ID,
			BrokerName: name,
			BrokerID:   strconv.Itoa(broker.ID),
			Host:       broker.Host,
			Port:       broker.Port,
			Role:       "broker",
			Status:     "online",
			Rack:       broker.Rack,
			LastSyncAt: &now,
		})
	}

	topicPartitions := make(map[string][]kafka.Partition)
	for _, partition := range partitions {
		if strings.HasPrefix(partition.Topic, "__") {
			continue
		}
		topicPartitions[partition.Topic] = append(topicPartitions[partition.Topic], partition)
	}
	for topic, list := range topicPartitions {
		replicaCount := 0
		if len(list) > 0 {
			replicaCount = len(list[0].Replicas)
		}
		snapshot.Resources = append(snapshot.Resources, &MQResource{
			InstanceID:     instance.ID,
			ResourceType:   ResourceTypeTopic,
			Name:           topic,
			FullName:       topic,
			PartitionCount: len(list),
			ReplicaCount:   replicaCount,
			MetadataJSON:   mustJSON(list),
			LastSyncAt:     &now,
		})
		for _, partition := range list {
			snapshot.Partitions = append(snapshot.Partitions, &MQPartition{
				InstanceID:   instance.ID,
				ResourceName: topic,
				PartitionID:  partition.ID,
				Leader:       brokerName(partition.Leader),
				ReplicasJSON: mustJSON(partition.Replicas),
				ISRJSON:      mustJSON(partition.Isr),
				Status:       partitionStatus(partition),
				LastSyncAt:   &now,
			})
		}
	}
	return snapshot, nil
}

func (a *KafkaAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	if req == nil || strings.TrimSpace(req.ResourceName) == "" {
		return nil, fmt.Errorf("Topic不能为空")
	}
	limit := req.Limit
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	brokers := kafkaBrokers(instance)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("Kafka bootstrap地址不能为空")
	}
	partition := 0
	if req.PartitionID != nil {
		partition = *req.PartitionID
	}
	startOffset := kafka.FirstOffset
	if req.Offset != nil {
		startOffset = *req.Offset
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       strings.TrimSpace(req.ResourceName),
		Partition:   partition,
		StartOffset: startOffset,
		MinBytes:    1,
		MaxBytes:    maxBytes,
		MaxWait:     2 * time.Second,
	})
	defer reader.Close()

	samples := make([]MessageSampleVO, 0, limit)
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for len(samples) < limit {
		message, err := reader.ReadMessage(readCtx)
		if err != nil {
			if len(samples) > 0 {
				break
			}
			return nil, fmt.Errorf("读取Kafka消息失败: %w", err)
		}
		payload, truncated, encoding := truncatePayload(message.Value, maxBytes)
		headers := make(map[string]string, len(message.Headers))
		for _, header := range message.Headers {
			value, _, _ := truncatePayload(header.Value, 512)
			headers[header.Key] = value
		}
		samples = append(samples, MessageSampleVO{
			Topic:       message.Topic,
			PartitionID: message.Partition,
			Offset:      message.Offset,
			Key:         string(message.Key),
			Timestamp:   message.Time.Format("2006-01-02 15:04:05"),
			Headers:     headers,
			Payload:     payload,
			PayloadSize: len(message.Value),
			Truncated:   truncated,
			Encoding:    encoding,
		})
	}
	return &MessageSampleResultVO{
		InstanceID:   instance.ID,
		MQType:       MQTypeKafka,
		ResourceName: strings.TrimSpace(req.ResourceName),
		Samples:      samples,
		SampleCount:  len(samples),
		Message:      "Kafka消息采样完成",
		SampledAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func kafkaDial(ctx context.Context, instance *MQInstance) (*kafka.Conn, error) {
	address := firstEndpoint(instance, DefaultPort(MQTypeKafka))
	if address == "" {
		return nil, fmt.Errorf("Kafka bootstrap地址不能为空")
	}
	dialer := &kafka.Dialer{Timeout: 10 * time.Second, ClientID: "opshub-messagequeue"}
	if instance.TLSEnabled {
		dialer.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("连接Kafka失败: %w", err)
	}
	return conn, nil
}

func kafkaBrokers(instance *MQInstance) []string {
	endpoints := splitEndpoints(instance.Endpoint)
	result := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		result = append(result, normalizeAddress(endpoint, DefaultPort(MQTypeKafka)))
	}
	return result
}

func brokerName(broker kafka.Broker) string {
	host := broker.Host
	if host == "" {
		host = "broker"
	}
	if broker.Port > 0 {
		host = net.JoinHostPort(host, strconv.Itoa(broker.Port))
	}
	return fmt.Sprintf("%s#%d", host, broker.ID)
}

func partitionStatus(partition kafka.Partition) string {
	if partition.Error != nil {
		return "error"
	}
	if len(partition.OfflineReplicas) > 0 {
		return "degraded"
	}
	return "online"
}
