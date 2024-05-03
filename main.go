package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/youthlin/go-lame"
)

var port = flag.String("p", "8888", "listen port")
var gid = flag.String("g", "group_fzm", "group id")
var score = flag.Float64("s", 0.36, "score threshold")

func main() {
	flag.Parse()
	httpServer()
}

func httpServer() {
	http.HandleFunc("/upload", uploadHandler)
	fmt.Println("Starting HTTP server...")
	err := http.ListenAndServe(":"+*port, nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("HTTP server stopped.")
}

func hpptsServer() {
	http.HandleFunc("/upload", uploadHandler)
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
	server := &http.Server{
		Addr:      ":443",
		TLSConfig: cfg,
	}

	fmt.Println("Starting HTTPS server...")
	err := server.ListenAndServeTLS("server.crt", "server.key")
	if err != nil {
		fmt.Println(err)
	}
}

func wav2mp3(b []byte) ([]byte, error) {
	rd := bytes.NewReader(b)
	wavHeader, err := lame.ReadWavHeader(rd)
	if err != nil {
		println("Failed to read wav header")
		return nil, err
	}

	buf := new(bytes.Buffer)
	wr, err := lame.NewWriter(buf)
	if err != nil {
		println("Failed to create lame writer")
		return nil, err
	}
	wr.EncodeOptions = wavHeader.ToEncodeOptions()
	wr.EncodeOptions.InBitsPerSample = 16
	wr.EncodeOptions.InSampleRate = 16000
	wr.EncodeOptions.InNumChannels = 1
	io.Copy(wr, rd)
	return buf.Bytes(), nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	println("process remote request:", r.RemoteAddr)

	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" {
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Missing address parameter", http.StatusBadRequest)
		println("Missing address parameter")
		return
	}
	featureInfo := address
	featureId := address
	if len(address) > 32 {
		featureId = address[:32]
	}

	println("featureId:", featureId, "featureInfo:", featureInfo, "address:", address)

	// 读取请求体
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		println("Failed to read request body")
		return
	}
	// wav to mp3
	buf, err := wav2mp3(b)
	if err != nil {
		http.Error(w, "Failed to convert wav to mp3", http.StatusInternalServerError)
		println("Failed to convert wav to mp3", len(buf))
		return
	}
	// mp3 data to base64 encode
	audio := base64.StdEncoding.EncodeToString(buf)

	// first, use searchScoreFea(1:1) to find featureId
	ri := &reqInfo{
		groupId:   *gid,
		apiName:   "searchScoreFea",
		featureId: featureId,
		audio:     audio,
	}

	res, code, err := vrg(ri)
	if err != nil {
		println("Failed to search srore feature", err.Error())
		if code != 23007 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// if featureId exists, checkin
	if res != nil && res.featureId == featureId {
		if res.score > *score { //  签到
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("yes, you are " + featureId))
		} else {
			http.Error(w, "no, you are not "+featureId, http.StatusBadRequest)
		}
		return
	}

	// second, use searchFea(1:N) to find featureId
	ri.apiName = "searchFea"
	res, code, err = vrg(ri)
	if err != nil {
		println("Failed to search feature", err.Error())
		if code != 23008 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if res != nil && res.featureId == featureId {
		println("can't go here, 1:1 not found, but 1:N found")
		http.Error(w, "can't go here, 1:1 not found, but 1:N found", http.StatusInternalServerError)
	}

	if res != nil && res.score >= *score {
		http.Error(w, "oh, you are "+res.featureId+" not "+featureId, http.StatusBadRequest)
		return
	}

	ri.apiName = "createFeature"
	ri.featureId = featureId
	ri.featureInfo = featureInfo

	_, _, err = vrg(ri)
	if err != nil {
		println("create feature error: ", err.Error())
		http.Error(w, "create feature error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("create new feature for you: " + featureId))
}
