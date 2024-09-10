// SPDX-License-Identifier: AGPL-3.0-only

package ingest

import (
	"math"
	"testing"
	"time"

	"github.com/grafana/dskit/flagext"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		setup       func(*Config)
		expectedErr error
	}{
		"should pass with the default config": {
			setup: func(_ *Config) {},
		},
		"should fail if ingest storage is enabled and Kafka address is not configured": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Topic = "test"
			},
			expectedErr: ErrMissingKafkaAddress,
		},
		"should fail if ingest storage is enabled and Kafka topic is not configured": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
			},
			expectedErr: ErrMissingKafkaTopic,
		},
		"should pass if ingest storage is enabled and required config is set": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
			},
		},
		"should fail if ingest storage is enabled and consume position is invalid": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ConsumeFromPositionAtStartup = "middle"
			},
			expectedErr: ErrInvalidConsumePosition,
		},
		"should fail if ingest storage is enabled and consume timestamp is set and consume position is not expected": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ConsumeFromPositionAtStartup = consumeFromEnd
				cfg.KafkaConfig.ConsumeFromTimestampAtStartup = time.Now().UnixMilli()
			},
			expectedErr: ErrInvalidConsumePosition,
		},
		"should fail if ingest storage is enabled and consume position is expected but consume timestamp is invalid": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ConsumeFromPositionAtStartup = consumeFromTimestamp
				cfg.KafkaConfig.ConsumeFromTimestampAtStartup = 0
			},
			expectedErr: ErrInvalidConsumePosition,
		},
		"should fail if ingest storage is enabled and the configured number of Kafka write clients is 0": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.WriteClients = 0
			},
			expectedErr: ErrInvalidWriteClients,
		},
		"should fail if ingest storage is enabled and producer max record size bytes is set too low": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ProducerMaxRecordSizeBytes = minProducerRecordDataBytesLimit - 1
			},
			expectedErr: ErrInvalidProducerMaxRecordSizeBytes,
		},
		"should fail if ingest storage is enabled and producer max record size bytes is set too high": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ProducerMaxRecordSizeBytes = maxProducerRecordDataBytesLimit + 1
			},
			expectedErr: ErrInvalidProducerMaxRecordSizeBytes,
		},
		"should fail if target consumer lag is enabled but max consumer lag is not": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.TargetConsumerLagAtStartup = 2 * time.Second
				cfg.KafkaConfig.MaxConsumerLagAtStartup = 0
			},
			expectedErr: ErrInconsistentConsumerLagAtStartup,
		},
		"should fail if max consumer lag is enabled but target consumer lag is not": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.TargetConsumerLagAtStartup = 0
				cfg.KafkaConfig.MaxConsumerLagAtStartup = 2 * time.Second
			},
			expectedErr: ErrInconsistentConsumerLagAtStartup,
		},
		"should fail if target consumer lag is > max consumer lag": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.TargetConsumerLagAtStartup = 2 * time.Second
				cfg.KafkaConfig.MaxConsumerLagAtStartup = 1 * time.Second
			},
			expectedErr: ErrInvalidMaxConsumerLagAtStartup,
		},
		"should fail if fetch max bytes is set too high": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ConsumerFetchMaxBytes = (math.MaxInt32 * 2) + 1
			},
			expectedErr: ErrInvalidConsumerFetchMaxBytes,
		},
		"should fail if fetch max partition bytes is set too high": {
			setup: func(cfg *Config) {
				cfg.Enabled = true
				cfg.KafkaConfig.Address = "localhost"
				cfg.KafkaConfig.Topic = "test"
				cfg.KafkaConfig.ConsumerFetchMaxPartitionBytes = math.MaxInt64
			},
			expectedErr: ErrInvalidConsumerFetchMaxPartitionBytes,
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			cfg := Config{}
			flagext.DefaultValues(&cfg)
			testData.setup(&cfg)

			assert.ErrorIs(t, cfg.Validate(), testData.expectedErr)
		})
	}
}

func TestConfig_GetConsumerGroup(t *testing.T) {
	tests := map[string]struct {
		consumerGroup string
		instanceID    string
		partitionID   int32
		expected      string
	}{
		"should return the instance ID if no consumer group is explicitly configured": {
			consumerGroup: "",
			instanceID:    "ingester-zone-a-1",
			partitionID:   1,
			expected:      "ingester-zone-a-1",
		},
		"should return the configured consumer group if set": {
			consumerGroup: "ingester-a",
			instanceID:    "ingester-zone-a-1",
			partitionID:   1,
			expected:      "ingester-a",
		},
		"should support <partition> placeholder in the consumer group": {
			consumerGroup: "ingester-zone-a-partition-<partition>",
			instanceID:    "ingester-zone-a-1",
			partitionID:   1,
			expected:      "ingester-zone-a-partition-1",
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			cfg := KafkaConfig{ConsumerGroup: testData.consumerGroup}
			assert.Equal(t, testData.expected, cfg.GetConsumerGroup(testData.instanceID, testData.partitionID))
		})
	}
}
