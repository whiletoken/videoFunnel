package main

import (
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

//go:embed static/*
var static embed.FS

var (
	threadingNumber int
	blockSize       int
	serverAddr      string
)

func main() {
	threadingNumber = *flag.Int("p", 4, "threading number")
	blockSize = *flag.Int("s", 1024*1024*1, "block size")
	serverAddr = *flag.String("addr", "0.0.0.0:9800", "listen address")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})
	http.Handle("/static/", http.FileServer(http.FS(static)))
	http.HandleFunc("/proxy", proxyServer)
	log.Printf("Video Funnel address: %s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}

func proxyServer(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		return
	}
	encoded := r.Form.Get("link")
	log.Printf("proxy request url: %s\n", encoded)

	// 解码
	urlEncode, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Fatal("解码失败:", err)
	}
	url := string(urlEncode)

	if r.Header.Get("Range") == "" {
		resp, err := http.Get(string(url))
		if err != nil {
			w.WriteHeader(resp.StatusCode)
			return
		}
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("ETag", resp.Header.Get("ETag"))
		w.WriteHeader(http.StatusOK)
		return
	}

	startPoint, endPoint := ParseRangePoint(r.Header.Get("Range"))
	contentLength := GetContentLength(string(url))
	if endPoint == -1 {
		endPoint = contentLength - 1
	}

	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", strconv.Itoa(endPoint-startPoint+1))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startPoint, endPoint, contentLength))
	w.WriteHeader(http.StatusPartialContent)

	channels := make([]chan []byte, threadingNumber)
	for i := 0; i < threadingNumber; i++ {
		channels[i] = make(chan []byte)
	}

	for startPoint <= endPoint {
		for i := 0; i < threadingNumber; i++ {
			tEndPoint := startPoint + blockSize
			if tEndPoint > endPoint {
				tEndPoint = endPoint
			}
			go GetVideoStream(startPoint, tEndPoint, endPoint, url, channels[i])
			startPoint = tEndPoint + 1 // 防止区间重合
		}
		for i := 0; i < threadingNumber; i++ {
			if t, ok := <-channels[i]; ok {
				if _, err := w.Write(t); err != nil {
					log.Println(err)
					return
				}
				log.Println("Write success")
			}
		}
	}
	log.Printf("over! %d - %d\n", startPoint, endPoint)
}
