package kafka

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// ==================== Avro Serializer / Deserializer ====================

func TestAvroSerializer(t *testing.T) {
	t.Parallel()

	t.Run("passthrough bytes", func(t *testing.T) {
		t.Parallel()
		s := &AvroSerializer{MarshalFunc: func(v interface{}) ([]byte, error) { return []byte("avro"), nil }}
		out, err := s.Serialize([]byte("raw"))
		require.NoError(t, err)
		assert.Equal(t, []byte("raw"), out)
	})

	t.Run("marshal via func", func(t *testing.T) {
		t.Parallel()
		s := &AvroSerializer{MarshalFunc: func(v interface{}) ([]byte, error) { return []byte("avro"), nil }}
		out, err := s.Serialize("data")
		require.NoError(t, err)
		assert.Equal(t, []byte("avro"), out)
	})

	t.Run("marshal func returns error", func(t *testing.T) {
		t.Parallel()
		s := &AvroSerializer{MarshalFunc: func(v interface{}) ([]byte, error) { return nil, errors.New("boom") }}
		_, err := s.Serialize("data")
		assert.Error(t, err)
	})

	t.Run("nil MarshalFunc", func(t *testing.T) {
		t.Parallel()
		s := &AvroSerializer{}
		_, err := s.Serialize("data")
		assert.Error(t, err)
	})
}

func TestAvroDeserializer(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal via func", func(t *testing.T) {
		t.Parallel()
		called := false
		d := &AvroDeserializer{UnmarshalFunc: func(data []byte, target interface{}) error {
			called = true
			return nil
		}}
		err := d.Deserialize([]byte("data"), nil)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("unmarshal func returns error", func(t *testing.T) {
		t.Parallel()
		d := &AvroDeserializer{UnmarshalFunc: func(data []byte, target interface{}) error {
			return errors.New("boom")
		}}
		err := d.Deserialize([]byte("data"), nil)
		assert.Error(t, err)
	})

	t.Run("nil UnmarshalFunc", func(t *testing.T) {
		t.Parallel()
		d := &AvroDeserializer{}
		err := d.Deserialize([]byte("data"), nil)
		assert.Error(t, err)
	})
}

// ==================== ProtobufDeserializer missing branch ====================

func TestProtobufDeserializerNoInterface(t *testing.T) {
	t.Parallel()
	d := &ProtobufDeserializer{}
	var target string
	err := d.Deserialize([]byte("data"), &target)
	assert.Error(t, err)
}

// ==================== ConsumerMessage methods ====================

func newConsumerMessage(key string, value []byte, headers []kafka.Header, partition int, offset int64, deser Deserializer) *ConsumerMessage {
	return &ConsumerMessage{
		message: kafka.Message{
			Key:       []byte(key),
			Value:     value,
			Headers:   headers,
			Partition: partition,
			Offset:    offset,
		},
		deserializer: deser,
	}
}

func TestConsumerMessageKey(t *testing.T) {
	t.Parallel()
	msg := newConsumerMessage("my-key", nil, nil, 0, 0, nil)
	assert.Equal(t, "my-key", msg.Key())
}

func TestConsumerMessagePartition(t *testing.T) {
	t.Parallel()
	msg := newConsumerMessage("", nil, nil, 3, 0, nil)
	assert.Equal(t, 3, msg.Partition())
}

func TestConsumerMessageOffset(t *testing.T) {
	t.Parallel()
	msg := newConsumerMessage("", nil, nil, 0, 42, nil)
	assert.Equal(t, int64(42), msg.Offset())
}

func TestConsumerMessageBody(t *testing.T) {
	t.Parallel()
	msg := newConsumerMessage("", []byte("hello"), nil, 0, 0, nil)
	assert.Equal(t, []byte("hello"), msg.Body())
}

func TestConsumerMessageHeader(t *testing.T) {
	t.Parallel()
	headers := []kafka.Header{{Key: "x-trace", Value: []byte("abc123")}}
	msg := newConsumerMessage("", nil, headers, 0, 0, nil)
	h := msg.Header()
	assert.Equal(t, "abc123", h["x-trace"])
}

func TestConsumerMessageBodyAs(t *testing.T) {
	t.Parallel()

	t.Run("with deserializer", func(t *testing.T) {
		t.Parallel()
		msg := newConsumerMessage("", []byte(`{"name":"test"}`), nil, 0, 0, &JSONDeserializer{})
		var out map[string]interface{}
		require.NoError(t, msg.BodyAs(&out))
		assert.Equal(t, "test", out["name"])
	})

	t.Run("fallback to JSON when no deserializer", func(t *testing.T) {
		t.Parallel()
		msg := newConsumerMessage("", []byte(`{"name":"fallback"}`), nil, 0, 0, nil)
		var out map[string]interface{}
		require.NoError(t, msg.BodyAs(&out))
		assert.Equal(t, "fallback", out["name"])
	})
}

// ==================== KafkaClient.Deserialize ====================

func TestKafkaClientDeserialize(t *testing.T) {
	t.Parallel()

	t.Run("with configured deserializer", func(t *testing.T) {
		t.Parallel()
		client := &KafkaClient{deserializer: &JSONDeserializer{}}
		msg := newConsumerMessage("", []byte(`{"name":"ok"}`), nil, 0, 0, nil)
		var out map[string]interface{}
		require.NoError(t, client.Deserialize(msg, &out))
		assert.Equal(t, "ok", out["name"])
	})

	t.Run("fallback to JSON when no deserializer", func(t *testing.T) {
		t.Parallel()
		client := &KafkaClient{}
		msg := newConsumerMessage("", []byte(`{"name":"fallback"}`), nil, 0, 0, nil)
		var out map[string]interface{}
		require.NoError(t, client.Deserialize(msg, &out))
		assert.Equal(t, "fallback", out["name"])
	})
}

// ==================== errors.go ====================

func TestTranslateByErrorMessage_AllBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected error
	}{
		{"connection reset by peer", ErrConnectionLost},
		{"connection closed", ErrConnectionLost},
		{"replica not available", ErrReplicaNotAvailable},
		{"sasl authentication failed", ErrAuthenticationFailed},
		{"authorization failed", ErrAuthorizationFailed},
		{"invalid credentials", ErrInvalidCredentials},
		{"unknown topic foo", ErrTopicNotFound},
		{"topic already exists", ErrTopicAlreadyExists},
		{"unknown partition 0", ErrPartitionNotFound},
		{"invalid partition", ErrInvalidPartition},
		{"not coordinator for group", ErrNotGroupCoordinator},
		{"invalid group id", ErrInvalidGroupID},
		{"unknown member id", ErrUnknownMemberID},
		{"invalid session timeout", ErrInvalidSessionTimeout},
		{"rebalance in progress", ErrRebalanceInProgress},
		{"offset out of range", ErrOffsetOutOfRange},
		{"invalid commit offset", ErrInvalidCommitOffset},
		{"record too large", ErrMessageTooLarge},
		{"invalid message", ErrInvalidMessage},
		{"invalid message size xyz", ErrInvalidMessage}, // "invalid message" branch matches first
		{"leader not available", ErrLeaderNotAvailable},
		{"not leader for partition", ErrNotLeaderForPartition},
		{"request timed out", ErrRequestTimedOut},
		{"timeout waiting", ErrRequestTimedOut},
		{"deadline exceeded", ErrContextDeadlineExceeded},
		{"network error", ErrNetworkError},
		{"dial tcp refused", ErrNetworkError},
		{"i/o timeout", ErrRequestTimedOut}, // "timeout" branch matches before "i/o timeout"
		{"producer fenced", ErrProducerFenced},
		{"invalid producer epoch", ErrInvalidProducerEpoch},
		{"transaction coordinator fenced", ErrTransactionCoordinatorFenced},
		{"invalid transaction state", ErrInvalidTransactionState},
		{"invalid txn state", ErrInvalidTxnState},
		{"unsupported version", ErrUnsupportedVersion},
		{"unsupported for message format", ErrUnsupportedForMessageFormat},
		{"invalid request", ErrInvalidRequest},
		{"invalid config", ErrInvalidConfig},
		{"invalid replication factor", ErrInvalidReplicationFactor},
		{"out of order sequence", ErrOutOfOrderSequence},
		{"duplicate sequence", ErrDuplicateSequence},
		{"context canceled", ErrContextCanceled},
		{"context cancelled", ErrContextCanceled},
	}

	k := &KafkaClient{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			result := k.TranslateError(fmt.Errorf("%s", tc.input))
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTranslateError_Nil(t *testing.T) {
	t.Parallel()
	k := &KafkaClient{}
	assert.Nil(t, k.TranslateError(nil))
}

func TestTranslateError_Unknown(t *testing.T) {
	t.Parallel()
	k := &KafkaClient{}
	orig := errors.New("some unknown kafka error xyz")
	result := k.TranslateError(orig)
	assert.Equal(t, orig, result)
}

func TestIsTemporaryError(t *testing.T) {
	t.Parallel()
	k := &KafkaClient{}
	assert.True(t, k.IsTemporaryError(ErrConnectionFailed))
	assert.True(t, k.IsTemporaryError(ErrRebalanceInProgress))
	assert.True(t, k.IsTemporaryError(ErrLeaderNotAvailable))
	assert.False(t, k.IsTemporaryError(ErrAuthenticationFailed))
}

// ==================== setup.go ====================

func TestCreateTLSConfig(t *testing.T) {
	t.Parallel()

	t.Run("insecure skip verify", func(t *testing.T) {
		t.Parallel()
		cfg := TLSConfig{InsecureSkipVerify: true}
		tlsCfg, err := createTLSConfig(cfg)
		require.NoError(t, err)
		assert.True(t, tlsCfg.InsecureSkipVerify)
	})

	t.Run("missing CA cert file", func(t *testing.T) {
		t.Parallel()
		cfg := TLSConfig{CACertPath: "/nonexistent/ca.crt"}
		_, err := createTLSConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("missing client cert file", func(t *testing.T) {
		t.Parallel()
		cfg := TLSConfig{ClientCertPath: "/nonexistent/client.crt", ClientKeyPath: "/nonexistent/client.key"}
		_, err := createTLSConfig(cfg)
		assert.Error(t, err)
	})
}

func TestCreateSASLMechanism(t *testing.T) {
	t.Parallel()

	t.Run("PLAIN", func(t *testing.T) {
		t.Parallel()
		m, err := createSASLMechanism(SASLConfig{Mechanism: "PLAIN", Username: "u", Password: "p"})
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("SCRAM-SHA-256", func(t *testing.T) {
		t.Parallel()
		m, err := createSASLMechanism(SASLConfig{Mechanism: "SCRAM-SHA-256", Username: "u", Password: "p"})
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("SCRAM-SHA-512", func(t *testing.T) {
		t.Parallel()
		m, err := createSASLMechanism(SASLConfig{Mechanism: "SCRAM-SHA-512", Username: "u", Password: "p"})
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("unsupported mechanism", func(t *testing.T) {
		t.Parallel()
		_, err := createSASLMechanism(SASLConfig{Mechanism: "GSSAPI"})
		assert.Error(t, err)
	})
}

func TestLogWarnLogError(t *testing.T) {
	t.Parallel()
	mock := &MockLogger{}
	k := &KafkaClient{logger: mock}

	k.logWarn(context.Background(), "warn msg", nil)
	assert.True(t, mock.WarnCalled)

	k.logError(context.Background(), "err msg", nil)
	assert.True(t, mock.ErrorCalled)
}

func TestLogWithNilLogger(t *testing.T) {
	t.Parallel()
	k := &KafkaClient{}
	// should not panic
	k.logWarn(context.Background(), "warn", nil)
	k.logError(context.Background(), "error", nil)
}

func TestCreateErrorLogger(t *testing.T) {
	t.Parallel()

	t.Run("with logger", func(t *testing.T) {
		t.Parallel()
		mock := &MockLogger{}
		k := &KafkaClient{logger: mock}
		logFn := createErrorLogger(k)
		logFn("something went wrong: %s", "detail")
		assert.True(t, mock.ErrorCalled)
	})

	t.Run("without logger no-op", func(t *testing.T) {
		t.Parallel()
		k := &KafkaClient{}
		logFn := createErrorLogger(k)
		// should not panic
		logFn("no logger here")
	})
}

func TestCreateWriter_Compression(t *testing.T) {
	t.Parallel()

	codecs := []string{"gzip", "snappy", "lz4", "zstd", ""}
	for _, codec := range codecs {
		codec := codec
		t.Run("codec="+codec, func(t *testing.T) {
			t.Parallel()
			cfg := Config{
				Brokers:          []string{"localhost:9092"},
				Topic:            "test",
				CompressionCodec: codec,
			}
			w := createWriter(cfg, nil, nil, &KafkaClient{})
			assert.NotNil(t, w)
			_ = w.Close()
		})
	}
}

func TestCreateWriter_Async(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Brokers:      []string{"localhost:9092"},
		Topic:        "test",
		Async:        true,
		BatchSize:    50,
		BatchTimeout: DefaultBatchTimeout,
	}
	w := createWriter(cfg, nil, nil, &KafkaClient{})
	assert.NotNil(t, w)
	_ = w.Close()
}

func TestNewClient_WithTLSError(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		TLS: TLSConfig{
			Enabled:    true,
			CACertPath: "/nonexistent/ca.crt",
		},
	}
	_, err := NewClient(cfg)
	assert.Error(t, err)
}

func TestNewClient_WithSASLError(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		SASL: SASLConfig{
			Enabled:   true,
			Mechanism: "UNSUPPORTED",
		},
	}
	_, err := NewClient(cfg)
	assert.Error(t, err)
}

func TestNewClient_Consumer(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Brokers:    []string{"localhost:9092"},
		Topic:      "test",
		GroupID:    "grp",
		IsConsumer: true,
	}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client.reader)
	assert.Nil(t, client.writer)
	client.GracefulShutdown()
}

// ==================== serializer.go ====================

func TestGetDefaultSerializer(t *testing.T) {
	t.Parallel()

	assert.IsType(t, &StringSerializer{}, getDefaultSerializer("string"))
	assert.IsType(t, &GobSerializer{}, getDefaultSerializer("gob"))
	assert.IsType(t, &NoOpSerializer{}, getDefaultSerializer("bytes"))
	assert.IsType(t, &JSONSerializer{}, getDefaultSerializer("json"))
	assert.IsType(t, &JSONSerializer{}, getDefaultSerializer(""))
	assert.IsType(t, &JSONSerializer{}, getDefaultSerializer("unknown"))
	assert.Nil(t, getDefaultSerializer("protobuf"))
	assert.Nil(t, getDefaultSerializer("avro"))
}

func TestGetDefaultDeserializer(t *testing.T) {
	t.Parallel()

	assert.IsType(t, &StringDeserializer{}, getDefaultDeserializer("string"))
	assert.IsType(t, &GobDeserializer{}, getDefaultDeserializer("gob"))
	assert.IsType(t, &NoOpDeserializer{}, getDefaultDeserializer("bytes"))
	assert.IsType(t, &JSONDeserializer{}, getDefaultDeserializer("json"))
	assert.IsType(t, &JSONDeserializer{}, getDefaultDeserializer(""))
	assert.IsType(t, &JSONDeserializer{}, getDefaultDeserializer("unknown"))
	assert.Nil(t, getDefaultDeserializer("protobuf"))
	assert.Nil(t, getDefaultDeserializer("avro"))
}

func TestValidateDataType(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateDataType("json", false, false))
	assert.NoError(t, ValidateDataType("string", false, false))
	assert.NoError(t, ValidateDataType("gob", false, false))
	assert.NoError(t, ValidateDataType("bytes", false, false))
	assert.NoError(t, ValidateDataType("", false, false))
	assert.NoError(t, ValidateDataType("unknown", false, false))

	assert.Error(t, ValidateDataType("protobuf", false, true))
	assert.Error(t, ValidateDataType("protobuf", true, false))
	assert.NoError(t, ValidateDataType("protobuf", true, true))

	assert.Error(t, ValidateDataType("avro", false, true))
	assert.Error(t, ValidateDataType("avro", true, false))
	assert.NoError(t, ValidateDataType("avro", true, true))
}

func TestMultiFormatSerializer_UnknownDefaultFormat(t *testing.T) {
	t.Parallel()
	s := &MultiFormatSerializer{DefaultFormat: "nonexistent", Serializers: map[string]Serializer{}}
	_, err := s.Serialize("data")
	assert.Error(t, err)
}

func TestMultiFormatDeserializer_UnknownDefaultFormat(t *testing.T) {
	t.Parallel()
	d := &MultiFormatDeserializer{DefaultFormat: "nonexistent", Deserializers: map[string]Deserializer{}}
	err := d.Deserialize([]byte("data"), nil)
	assert.Error(t, err)
}

func TestMultiFormatSerializer_NilSerializersMap(t *testing.T) {
	t.Parallel()
	s := &MultiFormatSerializer{}
	s.RegisterSerializer("custom", &JSONSerializer{})
	_, err := s.SerializeWithFormat("custom", map[string]string{"k": "v"})
	assert.NoError(t, err)
}

func TestMultiFormatDeserializer_NilDeserializersMap(t *testing.T) {
	t.Parallel()
	d := &MultiFormatDeserializer{}
	d.RegisterDeserializer("custom", &JSONDeserializer{})
	var out map[string]interface{}
	err := d.DeserializeWithFormat("custom", []byte(`{"k":"v"}`), &out)
	assert.NoError(t, err)
}

// ==================== fx_module.go ====================

func TestNewClientWithDI_Error(t *testing.T) {
	t.Parallel()
	params := KafkaParams{
		Config: Config{
			Brokers: []string{"localhost:9092"},
			Topic:   "test",
			TLS: TLSConfig{
				Enabled:    true,
				CACertPath: "/nonexistent/ca.crt",
			},
		},
	}
	_, err := NewClientWithDI(params)
	assert.Error(t, err)
}

func TestNewClientWithDI_WithOptionals(t *testing.T) {
	t.Parallel()
	mock := &MockLogger{}
	ser := &MockSerializer{}
	deser := &MockDeserializer{}

	params := KafkaParams{
		Config:       Config{Brokers: []string{"localhost:9092"}, Topic: "test"},
		Logger:       mock,
		Serializer:   ser,
		Deserializer: deser,
	}
	client, err := NewClientWithDI(params)
	require.NoError(t, err)
	assert.Equal(t, mock, client.logger)
	client.mu.RLock()
	assert.Equal(t, ser, client.serializer)
	assert.Equal(t, deser, client.deserializer)
	client.mu.RUnlock()
	client.GracefulShutdown()
}

func TestFXModule(t *testing.T) {
	t.Parallel()
	app := fxtest.New(t,
		FXModule,
		fx.Provide(func() Config {
			return Config{Brokers: []string{"localhost:9092"}, Topic: "test"}
		}),
	)
	app.RequireStart()
	app.RequireStop()
}
