// Package logger provides structured logging functionality for Go applications.
//
// The logger package is designed to provide a standardized logging approach
// with features such as log levels, contextual logging, distributed tracing integration,
// and flexible output formatting. It integrates with the fx dependency injection framework
// for easy incorporation into applications.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - Logger interface: Defines the contract for logging operations
//   - LoggerClient struct: Concrete implementation of the Logger interface
//   - NewLoggerClient constructor: Returns *LoggerClient (concrete type)
//   - FXModule: Provides both *LoggerClient and Logger interface for dependency injection
//
// Core Features:
//   - Structured logging with key-value pairs
//   - Support for multiple log levels (Debug, Info, Warning, Error, Fatal)
//   - Context-aware logging for request tracing
//   - Distributed tracing integration with OpenTelemetry
//   - Automatic trace and span ID extraction from context
//   - JSON output format with ISO8601 timestamps
//   - Output directed to stderr
//
// # Direct Usage (Without FX)
//
// For simple applications or tests, create a logger directly:
//
//	import "github.com/Abolfazl-Alemi/stdlib-lab/pkg/logger"
//
//	// Create a new logger (returns concrete *LoggerClient)
//	log := logger.NewLoggerClient(logger.Config{
//		Level:         logger.Info,
//		EnableTracing: true,
//		ServiceName:   "my-service",
//	})
//
//	// Log with structured fields (without context)
//	log.Info("User logged in", nil, map[string]interface{}{
//		"user_id": "12345",
//		"ip":      "192.168.1.1",
//	})
//
//	// Log with trace context (automatically includes trace_id and span_id)
//	log.InfoWithContext(ctx, "Processing request", nil, map[string]interface{}{
//		"request_id": "abc-123",
//	})
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface. You must supply a logger.Config to the
// dependency injection container:
//
//	import (
//		"github.com/Abolfazl-Alemi/stdlib-lab/pkg/logger"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		logger.FXModule, // Provides *LoggerClient and logger.Logger interface
//		fx.Provide(func() logger.Config {
//			return logger.Config{
//				Level:         logger.Info,
//				EnableTracing: true,
//				ServiceName:   "my-service",
//			}
//		}),
//		fx.Invoke(func(log *logger.LoggerClient) {
//			log.Info("Service started", nil)
//		}),
//		// ... other modules
//	)
//	app.Run()
//
// # Type Aliases in Consumer Code
//
// To simplify your code and avoid tight coupling, use type aliases:
//
//	package myapp
//
//	import stdLogger "github.com/Abolfazl-Alemi/stdlib-lab/pkg/logger"
//
//	// Use type alias to reference the Logger interface
//	type Logger = stdLogger.Logger
//
//	// Now use Logger throughout your codebase
//	func MyFunction(log Logger) {
//		log.Info("Processing", nil)
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Logging Levels
//
// Log level constants are defined as string constants in this package:
//
//	logger.Debug   // "debug"
//	logger.Info    // "info"
//	logger.Warning // "warning"
//	logger.Error   // "error"
//
// Example usage:
//
//	log.Debug("Debug message", nil)
//	log.Info("Info message", nil)
//	log.Warn("Warning message", nil)
//	log.Error("Error message", err)
//	log.Fatal("Fatal message", err) // calls os.Exit(1) after logging
//
// # Context-Aware Logging
//
//	log.DebugWithContext(ctx, "Debug with trace", nil)
//	log.InfoWithContext(ctx, "Info with trace", nil)
//	log.WarnWithContext(ctx, "Warning with trace", nil)
//	log.ErrorWithContext(ctx, "Error with trace", err)
//	log.FatalWithContext(ctx, "Fatal with trace", err) // calls os.Exit(1) after logging
//
// # Configuration
//
// The logger can be configured via environment variables:
//
//	ZAP_LOGGER_LEVEL=debug          # Log level (debug, info, warning, error)
//	LOGGER_ENABLE_TRACING=true      # Enable distributed tracing integration
//	LOGGER_CALLER_SKIP=1            # Number of stack frames to skip for caller reporting
//
// # Tracing Integration
//
// When tracing is enabled (EnableTracing: true), the logger will automatically
// extract trace and span IDs from the context and include them in log entries.
// This provides correlation between logs and distributed traces in your observability system.
//
// The following fields are automatically added to log entries when tracing is enabled
// and a valid span context is present:
//   - trace_id: The OpenTelemetry trace ID
//   - span_id: The OpenTelemetry span ID
//
// To use tracing, ensure your application has OpenTelemetry configured and pass
// context with active spans to the *WithContext logging methods.
//
// # Performance Considerations
//
// The logger is designed to be performant with minimal allocations. However,
// be mindful of excessive debug logging in production environments.
//
// # Thread Safety
//
// All methods on the Logger interface are safe for concurrent use by multiple
// goroutines.
package logger
