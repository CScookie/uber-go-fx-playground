package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

// main function is the entry point of the program
func main() {
	// Create a new Uber FX application
	fx.New(
		// Provide dependencies and configuration to the application
		fx.Provide(
			// HTTP server creation function
			NewHTTPServer,
			// Annotate the NewServeMux function with a ParamTag
			fx.Annotate(
				NewServeMux,
				fx.ParamTags(`group:"routes"`),
			),
			// Register handlers as routes
			AsRoute(NewEchoHandler),
			AsRoute(NewHelloHandler),
			// Register the Zap logger
			zap.NewExample,
		),
		// Invoke functions that need to run during application initialization
		fx.Invoke(func(*http.Server) {}),
		// Configure the logger for the application using Zap
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
	).Run() // Run the application
}

// NewHTTPServer creates a new HTTP server using provided dependencies
func NewHTTPServer(lc fx.Lifecycle, mux *http.ServeMux, log *zap.Logger) *http.Server {
	// Create a new HTTP server with a given ServeMux and logger
	srv := &http.Server{Addr: ":8080", Handler: mux}

	// Register lifecycle hooks for starting and stopping the server
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Start the HTTP server asynchronously
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			log.Info("Starting HTTP server at", zap.String("addr", srv.Addr))
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// Shutdown the HTTP server gracefully
			return srv.Shutdown(ctx)
		},
	})

	// Return the created HTTP server
	return srv
}

// Route is an interface for HTTP handlers with a Pattern method
type Route interface {
	http.Handler
	Pattern() string
}

// EchoHandler is a simple HTTP handler that echoes the request body
type EchoHandler struct {
	log *zap.Logger
}

// HelloHandler is an HTTP handler that responds with a greeting
type HelloHandler struct {
	log *zap.Logger
}

// Pattern returns the URL pattern for the EchoHandler
func (*EchoHandler) Pattern() string {
	return "/echo"
}

// Pattern returns the URL pattern for the HelloHandler
func (*HelloHandler) Pattern() string {
	return "/hello"
}

// NewHelloHandler creates a new HelloHandler instance
func NewHelloHandler(log *zap.Logger) *HelloHandler {
	return &HelloHandler{log: log}
}

// NewEchoHandler creates a new EchoHandler instance
func NewEchoHandler(log *zap.Logger) *EchoHandler {
	return &EchoHandler{log: log}
}

// ServeHTTP implements the HTTP handler for EchoHandler
func (h *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Copy the request body to the response writer
	if _, err := io.Copy(w, r.Body); err != nil {
		h.log.Warn("Failed to handle request", zap.Error(err))
	}
}

// ServeHTTP implements the HTTP handler for HelloHandler
func (h *HelloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("Failed to read request", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Respond with a greeting containing the request body
	if _, err := fmt.Fprintf(w, "Hello, %s\n", body); err != nil {
		h.log.Error("Failed to write response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// NewServeMux creates a new HTTP ServeMux and registers routes
func NewServeMux(routes []Route) *http.ServeMux {
	// Create a new ServeMux
	mux := http.NewServeMux()

	// Register each route in the ServeMux
	for _, route := range routes {
		mux.Handle(route.Pattern(), route)
	}

	// Return the created ServeMux
	return mux
}

// AsRoute is a utility function to annotate a function as a Route
func AsRoute(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(Route)),
		fx.ResultTags(`group:"routes"`),
	)
}
