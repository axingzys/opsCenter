package messagequeue

import (
	"context"
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

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
