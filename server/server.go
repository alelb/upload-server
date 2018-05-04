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
	"strconv"
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

	current := r.Header.Get("CURRENT_FILE_COUNTER")
	checksum := r.Header.Get("checksum")

	buf := readContent(r)
	s := buf.String()

	if !check(buf.Bytes(), checksum) {
		responseWithError(&w, http.StatusInternalServerError, message.ChecksumFail, "")
		return
	}

	// Create an struct pointer of type DataContainer struct
	protodata := new(pb.DataContainer)
	// Convert all the data retrieved into the DataContainer struct type
	err := proto.Unmarshal([]byte(s), protodata)
	if err != nil {
		responseWithError(&w, http.StatusInternalServerError, message.ParseError, err.Error())
		return
	}

	detectionUUID := protodata.GetDetection_UUID()
	createDirIfNotExist(fmt.Sprintf("../storage/%s", detectionUUID))

	filename := fmt.Sprintf("%s@%s", detectionUUID, current)
	err = ioutil.WriteFile(fmt.Sprintf("../storage/%s/%s", detectionUUID, filename), buf.Bytes(), 0644)
	if err != nil {
		responseWithError(&w, http.StatusInternalServerError, message.ErrorSlug, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

func checksum(content []byte) string {
	hasher := md5.New()
	hasher.Write(content)
	ck := hex.EncodeToString(hasher.Sum(nil))
	return ck
}

func check(content []byte, ck string) bool {
	return checksum(content) == ck
}

func responseWithError(w *http.ResponseWriter, codeNumber int, code string, text string) {
	(*w).WriteHeader(codeNumber)
	byt, err := json.Marshal(message.NewError(code, text))
	if err != nil {
		panic(err)
	}
	(*w).Write(byt)
}

func createDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}
