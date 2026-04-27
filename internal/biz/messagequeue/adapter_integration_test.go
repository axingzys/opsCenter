package messagequeue

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestAdaptersIntegration(t *testing.T) {
	if os.Getenv("OPSHUB_MQ_INTEGRATION") != "1" {
		t.Skip("set OPSHUB_MQ_INTEGRATION=1 to run MQ adapter integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cases := []struct {
		name       string
		adapter    Adapter
		instance   *MQInstance
		credential *ConnectionCredential
	}{
		{
			name:    "rabbitmq",
			adapter: NewRabbitMQAdapter(),
			instance: &MQInstance{
				Model:         gorm.Model{ID: 1},
				Name:          "rabbitmq-it",
				MQType:        MQTypeRabbitMQ,
				Endpoint:      envString("OPSHUB_TEST_RABBITMQ_ENDPOINT", "127.0.0.1:5672"),
				ManagementURL: envString("OPSHUB_TEST_RABBITMQ_MANAGEMENT_URL", "http://127.0.0.1:15672"),
				Status:        InstanceStatusEnabled,
			},
			credential: &ConnectionCredential{
				Username: envString("OPSHUB_TEST_RABBITMQ_USERNAME", "guest"),
				Password: envString("OPSHUB_TEST_RABBITMQ_PASSWORD", "guest"),
			},
		},
		{
			name:    "kafka",
			adapter: NewKafkaAdapter(),
			instance: &MQInstance{
				Model:    gorm.Model{ID: 2},
				Name:     "kafka-it",
				MQType:   MQTypeKafka,
				Endpoint: envString("OPSHUB_TEST_KAFKA_ENDPOINT", "127.0.0.1:9092"),
				Status:   InstanceStatusEnabled,
			},
		},
		{
			name:    "rocketmq",
			adapter: NewRocketMQAdapter(),
			instance: &MQInstance{
				Model:    gorm.Model{ID: 3},
				Name:     "rocketmq-it",
				MQType:   MQTypeRocketMQ,
				Endpoint: envString("OPSHUB_TEST_ROCKETMQ_ENDPOINT", "127.0.0.1:9876"),
				Status:   InstanceStatusEnabled,
			},
		},
		{
			name:    "activemq",
			adapter: NewActiveMQAdapter(),
			instance: &MQInstance{
				Model:         gorm.Model{ID: 4},
				Name:          "activemq-it",
				MQType:        MQTypeActiveMQ,
				Endpoint:      envString("OPSHUB_TEST_ACTIVEMQ_ENDPOINT", "127.0.0.1:61616"),
				ManagementURL: envString("OPSHUB_TEST_ACTIVEMQ_MANAGEMENT_URL", "http://127.0.0.1:8161"),
				Status:        InstanceStatusEnabled,
			},
			credential: &ConnectionCredential{
				Username: envString("OPSHUB_TEST_ACTIVEMQ_USERNAME", "admin"),
				Password: envString("OPSHUB_TEST_ACTIVEMQ_PASSWORD", "admin"),
			},
		},
		{
			name:    "pulsar",
			adapter: NewPulsarAdapter(),
			instance: &MQInstance{
				Model:         gorm.Model{ID: 5},
				Name:          "pulsar-it",
				MQType:        MQTypePulsar,
				Endpoint:      envString("OPSHUB_TEST_PULSAR_ENDPOINT", "127.0.0.1:6650"),
				ManagementURL: envString("OPSHUB_TEST_PULSAR_MANAGEMENT_URL", "http://127.0.0.1:8080"),
				Status:        InstanceStatusEnabled,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.TrimSpace(tc.instance.Endpoint) == "" {
				t.Skip("endpoint is empty")
			}
			result, err := tc.adapter.TestConnection(ctx, tc.instance, tc.credential)
			if err != nil {
				t.Fatalf("test connection: %v", err)
			}
			if result == nil || result.Engine == "" {
				t.Fatalf("empty connection result: %#v", result)
			}
			snapshot, err := tc.adapter.DiscoverMetadata(ctx, tc.instance, tc.credential)
			if err != nil {
				t.Fatalf("discover metadata: %v", err)
			}
			if snapshot == nil || snapshot.Engine == "" {
				t.Fatalf("empty metadata snapshot: %#v", snapshot)
			}
		})
	}
}

func TestAdapterOperationsIntegration(t *testing.T) {
	if os.Getenv("OPSHUB_MQ_INTEGRATION") != "1" {
		t.Skip("set OPSHUB_MQ_INTEGRATION=1 to run MQ adapter operation integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	suffix := time.Now().UnixNano()

	t.Run("rabbitmq", func(t *testing.T) {
		adapter := NewRabbitMQAdapter()
		instance := &MQInstance{
			Model:         gorm.Model{ID: 11},
			Name:          "rabbitmq-it",
			MQType:        MQTypeRabbitMQ,
			Endpoint:      envString("OPSHUB_TEST_RABBITMQ_ENDPOINT", "127.0.0.1:5672"),
			ManagementURL: envString("OPSHUB_TEST_RABBITMQ_MANAGEMENT_URL", "http://127.0.0.1:15672"),
			Status:        InstanceStatusEnabled,
		}
		credential := &ConnectionCredential{
			Username: envString("OPSHUB_TEST_RABBITMQ_USERNAME", "guest"),
			Password: envString("OPSHUB_TEST_RABBITMQ_PASSWORD", "guest"),
		}
		exchange := fmt.Sprintf("opshub.it.exchange.%d", suffix)
		queue := fmt.Sprintf("opshub.it.queue.%d", suffix)
		executeIntegrationOperation(t, ctx, adapter, instance, credential, &ResourceOperationRequest{
			Action:       OperationActionRabbitMQExchangeUpsert,
			Namespace:    "/",
			ResourceName: exchange,
			Params:       map[string]any{"type": "direct", "durable": true, "autoDelete": false},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, credential, &ResourceOperationRequest{
			Action:       OperationActionRabbitMQQueueUpsert,
			Namespace:    "/",
			ResourceName: queue,
			Params:       map[string]any{"durable": true, "autoDelete": false},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, credential, &ResourceOperationRequest{
			Action:       OperationActionRabbitMQBindingUpsert,
			Namespace:    "/",
			ResourceName: queue,
			Params:       map[string]any{"source": exchange, "destinationType": "queue", "routingKey": "opshub.it"},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, credential, &ResourceOperationRequest{
			Action:       OperationActionRabbitMQQueuePurge,
			Namespace:    "/",
			ResourceName: queue,
		})
		executeIntegrationOperation(t, ctx, adapter, instance, credential, &ResourceOperationRequest{
			Action:       OperationActionRabbitMQQueueDelete,
			Namespace:    "/",
			ResourceName: queue,
			Params:       map[string]any{"ifUnused": false, "ifEmpty": false},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, credential, &ResourceOperationRequest{
			Action:       OperationActionRabbitMQExchangeDelete,
			Namespace:    "/",
			ResourceName: exchange,
			Params:       map[string]any{"ifUnused": false},
		})
	})

	t.Run("kafka", func(t *testing.T) {
		adapter := NewKafkaAdapter()
		instance := &MQInstance{
			Model:    gorm.Model{ID: 12},
			Name:     "kafka-it",
			MQType:   MQTypeKafka,
			Endpoint: envString("OPSHUB_TEST_KAFKA_ENDPOINT", "127.0.0.1:9092"),
			Status:   InstanceStatusEnabled,
		}
		topic := fmt.Sprintf("opshub-it-topic-%d", suffix)
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionKafkaTopicCreate,
			ResourceName: topic,
			Params:       map[string]any{"partitions": 1, "replicationFactor": 1, "configs": map[string]any{"retention.ms": "600000"}},
		})
		time.Sleep(2 * time.Second)
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionKafkaPartitionsExpand,
			ResourceName: topic,
			Params:       map[string]any{"partitions": 2},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionKafkaTopicConfigUpdate,
			ResourceName: topic,
			Params:       map[string]any{"configs": map[string]any{"retention.ms": "900000"}},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionKafkaTopicDelete,
			ResourceName: topic,
		})
	})

	t.Run("pulsar", func(t *testing.T) {
		adapter := NewPulsarAdapter()
		instance := &MQInstance{
			Model:         gorm.Model{ID: 13},
			Name:          "pulsar-it",
			MQType:        MQTypePulsar,
			Endpoint:      envString("OPSHUB_TEST_PULSAR_ENDPOINT", "127.0.0.1:6650"),
			ManagementURL: envString("OPSHUB_TEST_PULSAR_MANAGEMENT_URL", "http://127.0.0.1:8080"),
			Status:        InstanceStatusEnabled,
		}
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionPulsarRetentionUpdate,
			ResourceName: "public/default",
			Params:       map[string]any{"retentionTimeInMinutes": 1440, "retentionSizeInMB": 1024},
		})
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionPulsarTTLUpdate,
			ResourceName: "public/default",
			Params:       map[string]any{"messageTTLInSeconds": 86400},
		})
		topic := fmt.Sprintf("persistent://public/default/opshub-it-topic-%d", suffix)
		baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypePulsar), instance.TLSEnabled)
		if err := doJSONRequestWithBody(ctx, httpClient(instance.TLSEnabled), http.MethodPut, baseURL+"/admin/v2/"+pulsarTopicPath(topic), nil, nil, nil); err != nil {
			t.Fatalf("create pulsar topic: %v", err)
		}
		executeIntegrationOperation(t, ctx, adapter, instance, nil, &ResourceOperationRequest{
			Action:       OperationActionPulsarTopicDelete,
			ResourceName: topic,
			Params:       map[string]any{"force": true},
		})
	})
}

func executeIntegrationOperation(t *testing.T, ctx context.Context, adapter ResourceOperationAdapter, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) {
	t.Helper()
	validation, err := adapter.ValidateOperation(ctx, instance, credential, req)
	if err != nil {
		t.Fatalf("validate %s: %v", req.Action, err)
	}
	if validation == nil || !validation.Supported {
		t.Fatalf("operation unsupported: %#v", validation)
	}
	req.Confirmed = true
	result, err := adapter.ApplyOperation(ctx, instance, credential, req)
	if err != nil {
		t.Fatalf("apply %s: %v", req.Action, err)
	}
	if result == nil || strings.TrimSpace(result.Message) == "" {
		t.Fatalf("empty operation result: %#v", result)
	}
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
