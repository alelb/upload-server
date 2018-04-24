package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
	"upload/message"

	pb "upload/proto"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/http2"
)

var logger *log.Logger

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
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
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

// Chain applies middlewares to a http.HandlerFunc
func Chain(f http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	for _, m := range middlewares {

		f = m(f)
	}
	return f
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func readContent(r *http.Request) *bytes.Buffer {

	var reader io.Reader

	if r.Header.Get("Content-Encoding") == "gzip" {
		gzipr, err := gzip.NewReader(r.Body)
		defer gzipr.Close()
		if err != nil {
			panic(err)
		}
		reader = gzipr
	} else {
		reader = r.Body
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	return buf

}

func handler(w http.ResponseWriter, r *http.Request) {

	sequence := r.Header.Get("sequence")
	checksum := r.Header.Get("checksum")

	if sequence == "" {
		panic("Error. Sequence header is missing")
	}
	if checksum == "" {
		panic("Error. checksum header is missing")
	}

	buf := readContent(r)
	s := buf.String()

	if !check(buf.Bytes(), checksum) {
		handleChecksumError(&w)
		return
	}

	// Create an struct pointer of type DataContainer struct
	protodata := new(pb.DataContainer)
	// Convert all the data retrieved into the DataContainer struct type
	errProto := proto.Unmarshal([]byte(s), protodata)
	checkError(errProto)

	detectionUUID := protodata.GetDetection_UUID()
	filename := fmt.Sprintf("%s@%s", detectionUUID, sequence)

	err := ioutil.WriteFile(fmt.Sprintf("../storage/%s", filename), buf.Bytes(), 0644)
	checkError(err)

	w.WriteHeader(http.StatusOK)
}

func main() {

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
		handler,
		c.Method("POST"),
		c.Logging(),
		c.ChecksumExists(),
		/* 		c.Checksum(), */
	),
	)
	srv.ListenAndServeTLS("cert.pem", "key.pem")

	<-ctx.Done()
	logger.Println("Server stopped")

}

func checksum(content []byte) string {
	hasher := md5.New()
	hasher.Write(content)
	ck := hex.EncodeToString(hasher.Sum(nil))
	return ck
}

func check(content []byte, ck string) bool {
	return checksum(content) == ck
}

func handleChecksumError(w *http.ResponseWriter) {
	e := message.NewError(http.StatusBadRequest, "A Checksum error occurred")
	b, err := json.Marshal(e)
	checkError(err)

	(*w).WriteHeader(http.StatusBadRequest)
	(*w).Write(b)
}
