package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
	"upload/message"
)

// Middleware is
type Middleware func(http.HandlerFunc) http.HandlerFunc

type controller struct {
	logger *log.Logger
}

func (c *controller) shutdown(ctx context.Context, srv *http.Server) context.Context {
	ctx, done := context.WithCancel(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		defer done()

		<-quit
		c.logger.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.SetKeepAlivesEnabled(false)
		if err := srv.Shutdown(ctx); err != nil {
			c.logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
	}()

	return ctx
}

// Logging logs all requests with its path and the time it took to process
func (c *controller) Logging() Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			// Do middleware things
			start := time.Now()
			defer func() { c.logger.Println(r.Method, r.URL.Path, time.Since(start)) }()

			// Call the next middleware/handler in chain
			f(w, r)
		}
	}
}

// Method ensures that url can only be requested with a specific method, else returns a 400 Bad Request
func (c *controller) Method(m string) Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			// Do middleware things
			if r.Method != m {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			// Call the next middleware/handler in chain
			f(w, r)
		}
	}
}

// Method ensures that the coming request has a valid checksum header
func (c *controller) ChecksumExists() Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			// Check the checksum header
			if r.Header.Get("checksum") == "" {
				c.logger.Println("Checksum does not exist")
				responseWithError(&w, http.StatusInternalServerError, message.MissingHeaderCRC, "")
				return
			}

			// Call the next middleware/handler in chain
			f(w, r)
		}
	}
}

// Method ensures that the coming request has a valid total file count header
func (c *controller) TotalFileCountExists() Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			// Check the total file count header
			if r.Header.Get("TOTAL_FILE_COUNT") == "" {
				c.logger.Println("Total file count does not exist")
				responseWithError(&w, http.StatusInternalServerError, message.MissingHeaderTotalFileCount, "")
				return
			}

			// Call the next middleware/handler in chain
			f(w, r)
		}
	}
}

// Method ensures that the coming request has a valid current file counter header
func (c *controller) CurrentFileCounterExists() Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			// Check the total file count header
			if r.Header.Get("CURRENT_FILE_COUNTER") == "" {
				c.logger.Println("Current file counter does not exist")
				responseWithError(&w, http.StatusInternalServerError, message.MissingHeaderCurrentFileCounter, "")
				return
			}

			// Call the next middleware/handler in chain
			f(w, r)
		}
	}
}

// Method ensure that the coming request has the current file counter within the total file count
func (c *controller) CountingCheck() Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			current, _ := strconv.Atoi(r.Header.Get("CURRENT_FILE_COUNTER"))
			count, _ := strconv.Atoi(r.Header.Get("TOTAL_FILE_COUNT"))

			if current > count {
				c.logger.Println("Counting error")
				responseWithError(&w, http.StatusInternalServerError, message.CountingError, "CURRENT_FILE_COUNTER greater than TOTAL_FILE_COUNTER")
				return
			}

			// Call the next middleware/handler in chain
			f(w, r)
		}
	}

}

func (c *controller) Checksum() Middleware {

	// Create a new Middleware
	return func(f http.HandlerFunc) http.HandlerFunc {

		// Define the http.HandlerFunc
		return func(w http.ResponseWriter, r *http.Request) {

			// Check the checksum header
			ck := r.Header.Get("checksum")

			buf := readContent(r)
			if !check(buf.Bytes(), ck) {
				c.logger.Println("Checksum error")
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			// Call the next middleware/handler in chain
			f(w, r)

		}
	}
}
