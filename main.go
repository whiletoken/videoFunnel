package main

import (
	"embed"
	_ "embed"
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
	threadingNumber = *flag.Int("p", 2, "threading number")
	blockSize = *flag.Int("s", 1024*1024*1, "block size")
	serverAddr = *flag.String("addr", "0.0.0.0:9800", "listen address")
	flag.Parse()

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		content, _ := static.ReadFile("static/index.html")
		_, _ = writer.Write(content)
	})
	http.Handle("/static/", http.FileServer(http.FS(static)))
	http.HandleFunc("/proxy", proxyServer) // 视频代理
	log.Printf("Video Funnel address: %s\n", serverAddr)
	err := http.ListenAndServe(serverAddr, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

// 视频代理
func proxyServer(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		return
	}
	encoded := r.Form.Get("link") // 获取实际请求的连接
	log.Printf("proxy request url : %s\n", encoded)

	// 解码
	urlEncode, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Fatal("解码失败:", err)
	}
	url := string(urlEncode)

	if r.Header.Get("Range") == "" { // 请求初始化
		resp, err := http.Get(url)
		if err != nil {
			w.WriteHeader(resp.StatusCode)
			return
		}
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
		w.Header().Set("Accept-Ranges", "bytes") // 表示允许分段传输
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("ETag", resp.Header.Get("ETag"))

		w.WriteHeader(http.StatusOK)
		return
	}

	startPoint, endPoint := ParseRangePoint(r.Header.Get("Range")) // 获取片段的开始点与结束点
	contentLength := GetContentLength(url)                         // 视频总长度
	if endPoint == -1 {
		endPoint = contentLength - 1
	}

	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", strconv.Itoa(endPoint-startPoint+1))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startPoint, endPoint, contentLength))
	w.WriteHeader(http.StatusPartialContent)

	var channels []chan []byte
	for i := 0; i < threadingNumber; i++ {
		channels = append(channels, make(chan []byte))
	}
	log.Printf("begin to prcess %d-%d\n", startPoint, endPoint)

	for startPoint < endPoint { // 需要对其所对应的十进制数字进行比较
		for i := 0; i < threadingNumber; i++ {
			tEndPoint := startPoint + blockSize
			if tEndPoint > endPoint {
				tEndPoint = endPoint
			}
			go GetVideoStream(startPoint, tEndPoint, endPoint, url, channels[i])
			startPoint = tEndPoint + 1 // 防止区间重合
		}
		for i := 0; i < threadingNumber; i++ {
			t, ok := <-channels[i]
			if !ok { // 如果当前管道关闭，则跳过
				continue
			}
			_, err := w.Write(t)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Write success")
		}
	}
	log.Printf("over! %d - %d\n", startPoint, endPoint)
}
