package schema_registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func newMockServer(t *testing.T, schemaID int, schema string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		switch r.URL.Path {
		case "/schemas/ids/1", "/schemas/ids/42", "/schemas/ids/99", "/schemas/ids/100":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"schema": schema})
		case "/subjects/test-subject/versions", "/subjects/test-value/versions",
			"/subjects/test-avro/versions", "/subjects/test-proto/versions",
			"/subjects/test-json-deser/versions":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": schemaID})
		case "/subjects/test-subject/versions/latest":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": schemaID, "version": 1, "schema": schema,
			})
		case "/compatibility/subjects/test-subject/versions/latest":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"is_compatible": true})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
		}
	}))
}

func newErrorServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error_code":404,"message":"not found"}`))
	}))
}

// ── WithLogger ────────────────────────────────────────────────────────────────

type captureLogger struct {
	infoCalled  bool
	warnCalled  bool
	errorCalled bool
}

func (c *captureLogger) InfoWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	c.infoCalled = true
}
func (c *captureLogger) WarnWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	c.warnCalled = true
}
func (c *captureLogger) ErrorWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	c.errorCalled = true
}

func TestWithLogger(t *testing.T) {
	t.Parallel()
	client := &Client{}
	logger := &captureLogger{}
	out := client.WithLogger(logger)
	assert.Equal(t, client, out)
	assert.Equal(t, logger, client.logger)
}

func TestLogMethods_WithLogger(t *testing.T) {
	t.Parallel()
	logger := &captureLogger{}
	client := &Client{logger: logger}
	ctx := context.Background()

	client.logInfo(ctx, "info", nil)
	client.logWarn(ctx, "warn", nil)
	client.logError(ctx, "error", nil, nil)

	assert.True(t, logger.infoCalled)
	assert.True(t, logger.warnCalled)
	assert.True(t, logger.errorCalled)
}

func TestLogMethods_NoLogger(t *testing.T) {
	t.Parallel()
	client := &Client{}
	ctx := context.Background()
	// must not panic
	client.logInfo(ctx, "info", nil)
	client.logWarn(ctx, "warn", nil)
	client.logError(ctx, "error", nil, nil)
}

// ── client error paths ────────────────────────────────────────────────────────

func TestGetSchemaByID_ErrorStatus(t *testing.T) {
	t.Parallel()
	srv := newErrorServer(t, http.StatusNotFound)
	defer srv.Close()

	c, err := NewClient(Config{URL: srv.URL})
	require.NoError(t, err)
	_, err = c.GetSchemaByID(1)
	assert.Error(t, err)
}

func TestGetLatestSchema_ErrorStatus(t *testing.T) {
	t.Parallel()
	srv := newErrorServer(t, http.StatusInternalServerError)
	defer srv.Close()

	c, err := NewClient(Config{URL: srv.URL})
	require.NoError(t, err)
	_, err = c.GetLatestSchema("test-subject")
	assert.Error(t, err)
}

func TestRegisterSchema_ErrorStatus(t *testing.T) {
	t.Parallel()
	srv := newErrorServer(t, http.StatusUnprocessableEntity)
	defer srv.Close()

	c, err := NewClient(Config{URL: srv.URL})
	require.NoError(t, err)
	_, err = c.RegisterSchema("test-subject", `{"type":"string"}`, "AVRO")
	assert.Error(t, err)
}

func TestCheckCompatibility_ErrorStatus(t *testing.T) {
	t.Parallel()
	srv := newErrorServer(t, http.StatusNotFound)
	defer srv.Close()

	c, err := NewClient(Config{URL: srv.URL})
	require.NoError(t, err)
	_, err = c.CheckCompatibility("test-subject", `{"type":"string"}`, "AVRO")
	assert.Error(t, err)
}

func TestCheckCompatibility_NonAVROType(t *testing.T) {
	t.Parallel()
	schema := `{"type":"string"}`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()

	c, err := NewClient(Config{URL: srv.URL})
	require.NoError(t, err)
	ok, err := c.CheckCompatibility("test-subject", schema, "JSON")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRegisterSchema_NonAVROType(t *testing.T) {
	t.Parallel()
	schema := `{"type":"string"}`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()

	c, err := NewClient(Config{URL: srv.URL})
	require.NoError(t, err)
	id, err := c.RegisterSchema("test-subject", schema, "JSON")
	require.NoError(t, err)
	assert.Equal(t, 1, id)
}

func TestGetSchemaByID_NetworkError(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Config{URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond})
	require.NoError(t, err)
	_, err = c.GetSchemaByID(1)
	assert.Error(t, err)
}

func TestGetLatestSchema_NetworkError(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Config{URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond})
	require.NoError(t, err)
	_, err = c.GetLatestSchema("subject")
	assert.Error(t, err)
}

func TestRegisterSchema_NetworkError(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Config{URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond})
	require.NoError(t, err)
	_, err = c.RegisterSchema("subject", `{"type":"string"}`, "AVRO")
	assert.Error(t, err)
}

func TestCheckCompatibility_NetworkError(t *testing.T) {
	t.Parallel()
	c, err := NewClient(Config{URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond})
	require.NoError(t, err)
	_, err = c.CheckCompatibility("subject", `{"type":"string"}`, "AVRO")
	assert.Error(t, err)
}

// ── Avro serializer/deserializer ──────────────────────────────────────────────

func TestNewAvroSerializer_MissingMarshalFunc(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, 1, `{"type":"string"}`)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	_, err := NewAvroSerializer(AvroSerializerConfig{
		Registry: c,
		Subject:  "test-avro",
		Schema:   `{"type":"string"}`,
	})
	assert.Error(t, err)
}

func TestAvroSerializer_Roundtrip(t *testing.T) {
	t.Parallel()
	schema := `{"type":"string"}`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	serializer, err := NewAvroSerializer(AvroSerializerConfig{
		Registry: c,
		Subject:  "test-avro",
		Schema:   schema,
		MarshalFunc: func(v interface{}) ([]byte, error) {
			return []byte(v.(string)), nil
		},
	})
	require.NoError(t, err)

	// serialize []byte passthrough
	encoded, err := serializer.Serialize([]byte("hello"))
	require.NoError(t, err)
	assert.Greater(t, len(encoded), 5)

	// serialize via MarshalFunc
	encoded2, err := serializer.Serialize("world")
	require.NoError(t, err)
	assert.Greater(t, len(encoded2), 5)
}

func TestNewAvroDeserializer_MissingUnmarshalFunc(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, 1, `{"type":"string"}`)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	_, err := NewAvroDeserializer(AvroDeserializerConfig{Registry: c})
	assert.Error(t, err)
}

func TestAvroDeserializer_Roundtrip(t *testing.T) {
	t.Parallel()
	schema := `{"type":"string"}`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	deserializer, err := NewAvroDeserializer(AvroDeserializerConfig{
		Registry: c,
		UnmarshalFunc: func(data []byte, target interface{}) error {
			*(target.(*[]byte)) = data
			return nil
		},
	})
	require.NoError(t, err)

	payload := []byte("avro-data")
	header := EncodeSchemaID(1)
	message := append(header, payload...)

	var result []byte
	err = deserializer.Deserialize(message, &result)
	require.NoError(t, err)
	assert.Equal(t, payload, result)
}

// ── Protobuf serializer/deserializer ─────────────────────────────────────────

func TestProtobufSerializer_Roundtrip(t *testing.T) {
	t.Parallel()
	schema := `syntax = "proto3"; message Msg { string name = 1; }`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	serializer, err := NewProtobufSerializer(ProtobufSerializerConfig{
		Registry: c,
		Subject:  "test-proto",
		Schema:   schema,
		MarshalFunc: func(v interface{}) ([]byte, error) {
			return v.([]byte), nil
		},
	})
	require.NoError(t, err)

	// []byte passthrough
	encoded, err := serializer.Serialize([]byte("proto-data"))
	require.NoError(t, err)
	assert.Greater(t, len(encoded), 5)

	// via MarshalFunc
	encoded2, err := serializer.Serialize([]byte("via-marshal"))
	require.NoError(t, err)
	assert.Greater(t, len(encoded2), 5)
}

func TestProtobufSerializer_NoMarshalFunc(t *testing.T) {
	t.Parallel()
	schema := `syntax = "proto3";`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	serializer, err := NewProtobufSerializer(ProtobufSerializerConfig{
		Registry: c,
		Subject:  "test-proto",
		Schema:   schema,
	})
	require.NoError(t, err)

	// non-[]byte without MarshalFunc should error
	_, err = serializer.Serialize("not-bytes")
	assert.Error(t, err)
}

func TestProtobufDeserializer_Roundtrip(t *testing.T) {
	t.Parallel()
	schema := `syntax = "proto3";`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	deserializer, err := NewProtobufDeserializer(ProtobufDeserializerConfig{
		Registry: c,
		UnmarshalFunc: func(data []byte, target interface{}) error {
			*(target.(*[]byte)) = data
			return nil
		},
	})
	require.NoError(t, err)

	payload := []byte("proto-payload")
	message := append(EncodeSchemaID(1), payload...)

	var result []byte
	err = deserializer.Deserialize(message, &result)
	require.NoError(t, err)
	assert.Equal(t, payload, result)
}

// ── JSON deserializer ─────────────────────────────────────────────────────────

func TestJSONDeserializer_Roundtrip(t *testing.T) {
	t.Parallel()
	schema := `{"type":"object"}`
	srv := newMockServer(t, 100, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	serializer, err := NewJSONSerializer(JSONSerializerConfig{
		Registry: c,
		Subject:  "test-json-deser",
		Schema:   schema,
	})
	require.NoError(t, err)

	deserializer, err := NewJSONDeserializer(JSONDeserializerConfig{Registry: c})
	require.NoError(t, err)

	original := map[string]string{"hello": "world"}
	encoded, err := serializer.Serialize(original)
	require.NoError(t, err)

	var decoded map[string]string
	err = deserializer.Deserialize(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

// ── WrapperSerializer validation ──────────────────────────────────────────────

func TestNewWrapperSerializer_Validation(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, 1, `{"type":"string"}`)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	_, err := NewWrapperSerializer(WrapperSerializerConfig{Subject: "s", Schema: "sc"})
	assert.Error(t, err) // missing registry

	_, err = NewWrapperSerializer(WrapperSerializerConfig{Registry: c, Schema: "sc"})
	assert.Error(t, err) // missing subject

	_, err = NewWrapperSerializer(WrapperSerializerConfig{Registry: c, Subject: "s"})
	assert.Error(t, err) // missing schema
}

func TestWrapperSerializer_NoInnerSerializer_NonBytes(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t, 1, `{"type":"string"}`)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	s, err := NewWrapperSerializer(WrapperSerializerConfig{
		Registry: c, Subject: "test-subject", Schema: `{"type":"string"}`,
	})
	require.NoError(t, err)

	_, err = s.Serialize("not-bytes-and-no-inner-serializer")
	assert.Error(t, err)
}

func TestWrapperDeserializer_NoInnerDeserializer_NonBytesTarget(t *testing.T) {
	t.Parallel()
	schema := `{"type":"string"}`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()
	c, _ := NewClient(Config{URL: srv.URL})

	d, err := NewWrapperDeserializer(WrapperDeserializerConfig{Registry: c})
	require.NoError(t, err)

	message := append(EncodeSchemaID(1), []byte("data")...)
	var target string // not *[]byte → should error
	err = d.Deserialize(message, &target)
	assert.Error(t, err)
}

// ── FX module ─────────────────────────────────────────────────────────────────

func TestFXModule(t *testing.T) {
	schema := `{"type":"string"}`
	srv := newMockServer(t, 1, schema)
	defer srv.Close()

	var client *Client
	app := fxtest.New(t,
		fx.Provide(func() Config {
			return Config{URL: srv.URL}
		}),
		FXModule,
		fx.Populate(&client),
	)
	app.RequireStart()
	t.Cleanup(func() { app.RequireStop() })

	require.NotNil(t, client)
	id, err := client.RegisterSchema("test-subject", schema, "AVRO")
	require.NoError(t, err)
	assert.Equal(t, 1, id)
}
