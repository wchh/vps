package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/youthlin/go-lame"
)

var port = flag.String("p", "8888", "listen port")
var gid = flag.String("g", "group_fzm", "group id")
var score = flag.Float64("s", 0.36, "score threshold")
var path = flag.String("f", "./audio_files", "audio file path")

func main() {
	flag.Parse()
	os.Mkdir(*path, 0755)
	httpServer()
}

func httpServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/upload/result", resultHandler)
	fmt.Println("Starting HTTP server...")
	err := http.ListenAndServe(":"+*port, mux)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("HTTP server stopped.")
}

func hpptsServer() {
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/result", resultHandler)
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

type UploadResult struct {
	ID        string // upload id
	Result    int    // 0 is create ok; 1 is recongition ok; others is error
	Error     string // error info
	Timestamp int    // seconds from 1970-1-1
}

func wav2mp3(b []byte) ([]byte, error) {
	rd := bytes.NewReader(b)
	wavHeader, err := lame.ReadWavHeader(rd)
	if err != nil {
		log.Println("Failed to read wav header")
		return nil, err
	}

	buf := new(bytes.Buffer)
	wr, err := lame.NewWriter(buf)
	if err != nil {
		log.Println("Failed to create lame writer")
		return nil, err
	}
	wr.EncodeOptions = wavHeader.ToEncodeOptions()
	wr.EncodeOptions.InBitsPerSample = 16
	wr.EncodeOptions.InSampleRate = 16000
	wr.EncodeOptions.InNumChannels = 1
	io.Copy(wr, rd)
	return buf.Bytes(), nil
}

var resultMap = make(map[string]*UploadResult)

func resultHandler(w http.ResponseWriter, r *http.Request) {
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
		log.Println("Missing address parameter")
		return
	}
	result, ok := resultMap[address]
	if !ok {
		result = &UploadResult{Result: -1, Error: "sorry, no result for the address:" + address}
	}
	data, err := json.Marshal(result)
	if err != nil {
		result = &UploadResult{Result: -1, Error: err.Error()}
	}
	w.Write(data)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
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
		log.Println("Missing address parameter")
		return
	}
	featureInfo := address
	featureId := address
	if len(address) > 32 {
		featureId = address[:32]
	}
	id := r.URL.Query().Get("id")
	language := r.URL.Query().Get("language")
	text := r.URL.Query().Get("text")

	result := &UploadResult{ID: id, Timestamp: int(time.Now().Unix())}
	defer func() {
		resultMap[address] = result
	}()

	log.Println("id:", id, "address:", address, "featureId:", featureId, "language:", language, "text:", text)

	// 读取请求体
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		log.Println("Failed to read request body")
		result.Result = 2
		result.Error = "failed to read request body"
		return
	}

	iat_result, err := do_iat(b, address, language)
	if err != nil {
		http.Error(w, "iat error", http.StatusInternalServerError)
		log.Println("iat error:", err.Error())
		result.Result = 2
		result.Error = "iat error " + err.Error()
		return
	}
	iat_result = strings.ReplaceAll(iat_result, " ", "")
	text = strings.ReplaceAll(text, " ", "")
	if !strings.Contains(iat_result, text) {
		http.Error(w, "iat result is "+iat_result+" not match "+text, http.StatusBadRequest)
		result.Result = 2
		result.Error = "iat result is " + iat_result + " not match " + text
		return
	}

	// wav to mp3
	buf, err := wav2mp3(b)
	if err != nil {
		http.Error(w, "Failed to convert wav to mp3", http.StatusInternalServerError)
		log.Println("Failed to convert wav to mp3", len(buf))
		result.Result = 2
		result.Error = "failed to convert wav to mp3"
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
		log.Println("Failed to search srore feature", err.Error())
		if code != 23007 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			result.Result = 2
			result.Error = err.Error()
			return
		}
	}

	// if featureId exists, checkin
	if res != nil && res.featureId == featureId {
		if res.score > *score { //  签到
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("yes, you are " + featureId))
			result.Result = 1
		} else {
			http.Error(w, "no, you are not "+featureId, http.StatusBadRequest)
			result.Error = "you are not " + featureId
			result.Result = 2
		}
		return
	}

	// second, use searchFea(1:N) to find featureId
	ri.apiName = "searchFea"
	res, code, err = vrg(ri)
	if err != nil {
		log.Println("Failed to search feature", err.Error())
		if code != 23008 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			result.Result = 2
			result.Error = err.Error()
			return
		}
	}

	if res != nil && res.featureId == featureId {
		log.Println("can't go here, 1:1 not found, but 1:N found")
		http.Error(w, "can't go here, 1:1 not found, but 1:N found", http.StatusInternalServerError)
		result.Result = 2
		result.Error = "server error"
		return
	}

	if res != nil && res.score >= *score {
		http.Error(w, "oh, you are "+res.featureId+" not "+featureId, http.StatusBadRequest)
		result.Result = 2
		result.Error = "you are " + res.featureId + " not " + featureId
		return
	}

	ri.apiName = "createFeature"
	ri.featureId = featureId
	ri.featureInfo = featureInfo

	_, _, err = vrg(ri)
	if err != nil {
		log.Println("create feature error: ", err.Error())
		http.Error(w, "create feature error: "+err.Error(), http.StatusInternalServerError)
		result.Result = 2
		result.Error = err.Error()
		return
	}
	w.Write([]byte("create new feature for you: " + featureId))
	result.Result = 0
}

func do_iat(audio_buf []byte, address, language string) (string, error) {
	// save wav to file
	t := time.Now().Unix()
	fp := filepath.Join(*path, address+"_"+strconv.Itoa(int(t))+".wav")
	err := os.WriteFile(fp, audio_buf, 0666)
	if err != nil {
		log.Println(err)
		return "", err
	}
	fp_pcm := fp[:len(fp)-4] + ".pcm"

	// ffmpeg -y -i test.wav -acodec pcm_s16le -f s16le -ac 1 -ar 16000 test.pcm
	cmd := fmt.Sprintf("ffmpeg -y -i %s -acodec pcm_s16le -f s16le -ac 1 -ar 16000 %s", fp, fp_pcm)
	_, err = exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return "", err
	}
	if language == "" || language == "zh" {
		language = "zh_cn"
	} else {
		language = "en_us"
	}

	return iat(fp_pcm, language)
}
