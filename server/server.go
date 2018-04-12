package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	pb "upload/proto"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/http2"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {

	fmt.Println("accept connection")

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
	s := buf.String()

	// Create an struct pointer of type DataContainer struct
	protodata := new(pb.DataContainer)
	// Convert all the data retrieved into the DataContainer struct type
	errProto := proto.Unmarshal([]byte(s), protodata)
	check(errProto)

	detectionUUID := protodata.GetDetection_UUID()
	sessionUUID := protodata.GetSession_UUID()

	fmt.Printf("%v %v\n", detectionUUID, sessionUUID)

	err := ioutil.WriteFile(fmt.Sprintf("../storage/%s", detectionUUID), buf.Bytes(), 0644)
	check(err)

}

func main() {

	var srv http.Server
	// http2.VerboseLogs = true
	srv.Addr = ":8282"

	// This enables http2 support
	http2.ConfigureServer(&srv, &http2.Server{
		MaxHandlers: 0,
	})

	http.HandleFunc("/up", handler)
	srv.ListenAndServeTLS("cert.pem", "key.pem")

}
