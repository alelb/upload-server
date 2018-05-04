package server

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"upload/message"
	pb "upload/proto"

	"github.com/golang/protobuf/proto"
)

func up(w http.ResponseWriter, r *http.Request) {

	logger.Println("Accept connection")

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

func checksum(content []byte) string {
	hasher := md5.New()
	hasher.Write(content)
	ck := hex.EncodeToString(hasher.Sum(nil))
	return ck
}

func check(content []byte, ck string) bool {
	return checksum(content) == ck
}
