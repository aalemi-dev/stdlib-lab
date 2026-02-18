package kafka

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

// TestKafkaPublish verifies that publishing a message to Kafka works correctly.
func TestKafkaPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	brokers, containerInstance := initializeKafka(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *KafkaClient

	cfg := Config{
		Brokers:    brokers,
		Topic:      "test-topic",
		IsConsumer: false,
	}

	app := fx.New(
		FXModule,
		fx.Provide(func() Config { return cfg }),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer func() {
		if err := app.Stop(ctx); err != nil {
			t.Logf("failed to stop app: %v", err)
		}
	}()

	time.Sleep(5 * time.Second)

	msgBody := []byte(`{"event":"test-publish"}`)
	err := client.Publish(ctx, "test-key", msgBody)
	require.NoError(t, err)

	t.Log("Message published successfully")
}

// TestKafkaConsumeWithCommit verifies that a Kafka client can consume and commit a message.
func TestKafkaConsumeWithCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	brokers, containerInstance := initializeKafka(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	producerCfg := Config{
		Brokers:    brokers,
		Topic:      "test-topic",
		IsConsumer: false,
	}

	var producer *KafkaClient
	producerApp := fx.New(
		FXModule,
		fx.Provide(func() Config { return producerCfg }),
		fx.Populate(&producer),
	)

	require.NoError(t, producerApp.Start(ctx))
	defer func() {
		if err := producerApp.Stop(ctx); err != nil {
			t.Logf("failed to stop producer app: %v", err)
		}
	}()

	time.Sleep(5 * time.Second)

	consumerCfg := Config{
		Brokers:    brokers,
		Topic:      "test-topic",
		GroupID:    "test-group",
		IsConsumer: true,
	}

	var consumer *KafkaClient
	consumerApp := fx.New(
		FXModule,
		fx.Provide(func() Config { return consumerCfg }),
		fx.Populate(&consumer),
	)

	require.NoError(t, consumerApp.Start(ctx))
	defer func() {
		if err := consumerApp.Stop(ctx); err != nil {
			t.Logf("failed to stop consumer app: %v", err)
		}
	}()

	time.Sleep(3 * time.Second)

	wg := &sync.WaitGroup{}
	consumeCtx, consumeCancel := context.WithCancel(ctx)
	defer consumeCancel()

	msgs := consumer.Consume(consumeCtx, wg)
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range msgs {
			t.Logf("Message consumed successfully: %v", string(msg.Body()))
			if err := msg.CommitMsg(); err != nil {
				errCh <- fmt.Errorf("failed to commit message: %w", err)
				return
			}
			t.Log("Message committed successfully")
			errCh <- nil
			return //nolint:staticcheck
		}
	}()

	msgBody := []byte(`{"event":"test-consume"}`)
	err := producer.Publish(ctx, "test-key", msgBody)
	require.NoError(t, err)
	t.Log("Message published successfully")

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for message to be consumed")
	}

	consumeCancel()
	wg.Wait()
}

// TestKafkaPublishWithHeaders verifies that publishing a message with headers works correctly.
func TestKafkaPublishWithHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	brokers, containerInstance := initializeKafka(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	producerCfg := Config{
		Brokers:    brokers,
		Topic:      "test-topic",
		IsConsumer: false,
	}

	var producer *KafkaClient
	producerApp := fx.New(
		FXModule,
		fx.Provide(func() Config { return producerCfg }),
		fx.Populate(&producer),
	)

	require.NoError(t, producerApp.Start(ctx))
	defer func() {
		if err := producerApp.Stop(ctx); err != nil {
			t.Logf("failed to stop producer app: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)

	consumerCfg := Config{
		Brokers:    brokers,
		Topic:      "test-topic",
		GroupID:    "test-group-headers",
		IsConsumer: true,
	}

	var consumer *KafkaClient
	consumerApp := fx.New(
		FXModule,
		fx.Provide(func() Config { return consumerCfg }),
		fx.Populate(&consumer),
	)

	require.NoError(t, consumerApp.Start(ctx))
	defer func() {
		if err := consumerApp.Stop(ctx); err != nil {
			t.Logf("failed to stop consumer app: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)

	wg := &sync.WaitGroup{}
	consumeCtx, consumeCancel := context.WithCancel(ctx)
	defer consumeCancel()

	msgs := consumer.Consume(consumeCtx, wg)
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range msgs {
			t.Logf("Message consumed: %v", string(msg.Body()))
			headers := msg.Header()
			t.Logf("Headers: %v", headers)

			if headers["trace-id"] != "12345" {
				errCh <- fmt.Errorf("expected trace-id to be 12345, got %v", headers["trace-id"])
				return
			}

			if err := msg.CommitMsg(); err != nil {
				errCh <- fmt.Errorf("failed to commit message: %w", err)
				return
			}
			t.Log("Message with headers committed successfully")
			errCh <- nil
			return //nolint:staticcheck
		}
	}()

	headers := map[string]interface{}{
		"trace-id": "12345",
		"span-id":  "67890",
	}
	msgBody := []byte(`{"event":"test-headers"}`)
	err := producer.Publish(ctx, "test-key", msgBody, headers)
	require.NoError(t, err)
	t.Log("Message with headers published successfully")

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for message to be consumed")
	}

	consumeCancel()
	wg.Wait()
}

// TestKafkaConsumerContextCancellation verifies that the Kafka consumer correctly handles
// context cancellation during message consumption.
func TestKafkaConsumerContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	brokers, containerInstance := initializeKafka(ctx, t)
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	var client *KafkaClient

	cfg := Config{
		Brokers:    brokers,
		Topic:      "test-topic",
		GroupID:    "test-group-cancel",
		IsConsumer: true,
	}

	app := fx.New(
		FXModule,
		fx.Provide(func() Config { return cfg }),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer func() {
		if err := app.Stop(ctx); err != nil {
			t.Logf("failed to stop app: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)

	consumeCtx, consumeCancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	outChan := client.Consume(consumeCtx, wg)

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Second)
		consumeCancel()
	}()

	select {
	case _, ok := <-outChan:
		if ok {
			t.Fatal("Expected channel to be closed after context cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for consumer to stop after context cancel")
	}

	wg.Wait()
}

// TestKafkaErrorTranslation verifies error translation works correctly.
func TestKafkaErrorTranslation(t *testing.T) {
	k := &KafkaClient{}

	tests := []struct {
		name     string
		input    string
		expected error
	}{
		{"connection refused", "connection refused", ErrConnectionFailed},
		{"broker not available", "broker not available", ErrBrokerNotAvailable},
		{"authentication failed", "authentication failed", ErrAuthenticationFailed},
		{"topic not found", "topic not found", ErrTopicNotFound},
		{"offset out of range", "offset out of range", ErrOffsetOutOfRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := k.TranslateError(fmt.Errorf("%s", tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestKafkaErrorClassification verifies error classification methods.
func TestKafkaErrorClassification(t *testing.T) {
	k := &KafkaClient{}

	assert.True(t, k.IsRetryableError(ErrConnectionFailed))
	assert.True(t, k.IsRetryableError(ErrBrokerNotAvailable))
	assert.False(t, k.IsRetryableError(ErrAuthenticationFailed))

	assert.True(t, k.IsPermanentError(ErrAuthenticationFailed))
	assert.True(t, k.IsPermanentError(ErrTopicNotFound))
	assert.False(t, k.IsPermanentError(ErrConnectionFailed))

	assert.True(t, k.IsAuthenticationError(ErrAuthenticationFailed))
	assert.True(t, k.IsAuthenticationError(ErrInvalidCredentials))
	assert.False(t, k.IsAuthenticationError(ErrConnectionFailed))
}

func initializeKafka(ctx context.Context, t *testing.T) ([]string, testcontainers.Container) {
	t.Helper()

	hostPort, err := getFreePort()
	require.NoError(t, err)

	containerInstance, err := createKafkaContainer(ctx, hostPort)
	require.NoError(t, err)

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	require.Eventually(t, func() bool {
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("localhost", hostPort))
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "Kafka port not ready")

	brokers := []string{fmt.Sprintf("localhost:%s", hostPort)}
	createTestTopic(t, brokers, "test-topic")

	return brokers, containerInstance
}

// createTestTopic creates a test topic using kafka-go admin operations.
func createTestTopic(t *testing.T, brokers []string, topic string) {
	t.Helper()

	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		t.Logf("Warning: Could not create admin connection: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("failed to close admin connection: %v", err)
		}
	}()

	controller, err := conn.Controller()
	if err != nil {
		t.Logf("Warning: Could not get controller: %v", err)
		return
	}

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		t.Logf("Warning: Could not connect to controller: %v", err)
		return
	}
	defer func() {
		if err := controllerConn.Close(); err != nil {
			t.Logf("failed to close controller connection: %v", err)
		}
	}()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil {
		t.Logf("Warning: Could not create topic (may already exist): %v", err)
	} else {
		t.Logf("Created topic: %s", topic)
	}
}

func createKafkaContainer(ctx context.Context, hostPort string) (testcontainers.Container, error) {
	portBindings := nat.PortMap{
		"9092/tcp": []nat.PortBinding{{HostPort: hostPort}},
	}

	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:7.5.0",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_BROKER_ID":                                "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":           "PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":                     fmt.Sprintf("PLAINTEXT://localhost:29092,PLAINTEXT_HOST://localhost:%s", hostPort),
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
			"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS":         "0",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
			"KAFKA_PROCESS_ROLES":                            "broker,controller",
			"KAFKA_NODE_ID":                                  "1",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":                 "1@localhost:29093",
			"KAFKA_LISTENERS":                                "PLAINTEXT://0.0.0.0:29092,PLAINTEXT_HOST://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":               "PLAINTEXT",
			"KAFKA_CONTROLLER_LISTENER_NAMES":                "CONTROLLER",
			"KAFKA_LOG_DIRS":                                 "/tmp/kraft-combined-logs",
			"CLUSTER_ID":                                     "MkU3OEVBNTcwNTJENDM2Qk",
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE":                "true",
		},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.PortBindings = portBindings
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("9092/tcp").WithStartupTimeout(60*time.Second),
			wait.ForLog("Kafka Server started").WithStartupTimeout(60*time.Second),
		),
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err == nil {
			return c, nil
		}
		lastErr = err
		if strings.Contains(err.Error(), "docker.sock") {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		break
	}

	return nil, fmt.Errorf("failed to start Kafka container after 3 attempts: %w", lastErr)
}

func getFreePort() (string, error) {
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = l.Close() }()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}
