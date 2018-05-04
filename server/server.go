package server

import (
	"context"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/http2"
)

var logger *log.Logger

// Chain applies middlewares to a http.HandlerFunc
func Chain(f http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	for _, m := range middlewares {

		f = m(f)
	}
	return f
}

// Run the server
func Run(host string, port string) {

	logger = log.New(os.Stdout, "http: ", log.LstdFlags)
	logger.Println("Server is starting...")

	c := &controller{
		logger: logger,
	}

	srv := &http.Server{
		Addr: ":8282",
	}
	//http2.VerboseLogs = true

	// This enables http2 support
	http2.ConfigureServer(srv, &http2.Server{
		MaxHandlers: 0,
	})

	ctx := c.shutdown(context.Background(), srv)

	logger.Println("Server is ready to handle requests at", srv.Addr)
	http.HandleFunc("/up", Chain(
		up,
		c.Method("POST"),
		c.Logging(),
		c.ChecksumExists(),
		c.TotalFileCountExists(),
		c.CurrentFileCounterExists(),
		c.CountingCheck(),
		/* 		c.Checksum(), */
	),
	)
	srv.ListenAndServeTLS("cert.pem", "key.pem")

	<-ctx.Done()
	logger.Println("Server stopped")
}
