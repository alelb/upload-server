package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

type Package struct {
	content  []byte
	sequence string
	checksum string
}

func mkClient() *http.Client {

	client := &http.Client{
		Transport: &http2.Transport{
			DisableCompression: false,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return client
}

func listFiles(dirname string) []os.FileInfo {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}

	return files
}

func loadFiles(dirname string) []Package {

	files := listFiles(dirname)
	l := len(files)

	var arr = make([]Package, l)

	for i, f := range files {
		var err error
		content, err := ioutil.ReadFile(filepath.Join(dirname, f.Name()))
		if err != nil {
			panic(err)
		}
		name := f.Name()
		sequence := getSequence(name)
		arr[i] = newPackage(content, sequence)
	}

	return arr
}

func uploadDirectory(files []Package, duration time.Duration, compression bool) int64 {

	client := mkClient()
	l := len(files)

	var sleep time.Duration
	if duration > 0 {
		sleep = time.Duration(math.Floor(float64(duration) / float64(l)))
	}

	total := make(chan int64)
	chunk := make(chan int64)

	go func() {
		var bytes int64
		for i := 0; i < l; i++ {
			bytes += <-chunk
			// log.Println("cumulative: ", bytes)
		}
		total <- bytes
	}()

	for _, f := range files {
		time.Sleep(sleep)
		go func(f Package) {
			bytes := upload(client, f.content, compression, f.sequence, f.checksum)
			// log.Println("uploaded bytes: ", bytes)
			chunk <- bytes
		}(f)
	}

	return <-total
}

func upload(client *http.Client, content []byte, compression bool, sequence string, checksum string) int64 {

	var reader io.Reader

	if compression {
		piper, pipew := io.Pipe()

		buf := bytes.NewBuffer(content)

		gzipw := gzip.NewWriter(pipew)

		go func() {
			buf.WriteTo(gzipw)
			gzipw.Close()
			pipew.Close()
		}()
		defer piper.Close()
		reader = piper
	} else {
		reader = bytes.NewReader(content)
	}

	req, err := http.NewRequest("POST", "https://127.0.0.1:8282/up", reader)
	req.Header.Set("TOTAL_FILE_COUNT", "17")
	req.Header.Set("CURRENT_FILE_COUNTER", sequence)
	req.Header.Set("checksum", checksum)

	if err != nil {
		panic(err)
	}

	if compression {
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	// dreq, _ := httputil.DumpRequest(req, false)

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	// dres, _ := httputil.DumpResponse(res, true)

	// fmt.Print(string(dreq))
	// fmt.Print(string(dres))
	// fmt.Println()

	mex, _ := ioutil.ReadAll(res.Body)
	ret, _ := strconv.ParseInt(string(mex), 10, 64)

	return ret
}

func main() {
	if len(os.Args) <= 1 {
		return
	}

	dir := os.Args[1]
	/* 	files := loadFiles(dir)

	   	client := mkClient()

	   	upload(client, files[0], true) */

	uploadDirectory(loadFiles(dir), 0, true)

	/* 	resp, err := client.Get("https://localhost:8282/")
	   	if err != nil {
	   		log.Fatal(err)
	   	}

	   	body, err := ioutil.ReadAll(resp.Body)
	   	if err != nil {
	   		log.Fatal(err)
	   	}

	   	fmt.Println(string(body)) */
}

func checksum(content []byte) string {
	hasher := md5.New()
	hasher.Write(content)
	ck := hex.EncodeToString(hasher.Sum(nil))

	return ck
}

func newPackage(content []byte, sequence string) Package {

	p := Package{
		content:  content,
		sequence: sequence,
		checksum: checksum(content),
	}

	return p
}

func getSequence(name string) string {

	a := strings.Split(name, "@")
	if len(a) != 2 {
		panic("Error")
	}

	return a[1]
}
