package main

import (
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

//go:embed static/*
var static embed.FS

// threadingNumber 是一个命令行标志，用于指定线程数。
var threadingNumber = flag.Int("p", 4, "线程数")

// blockSize 是一个命令行标志，用于指定每个线程处理的块大小。
var blockSize = flag.Int("s", 1024*1024*1, "块大小")

// serverAddr 是一个命令行标志，用于指定服务器监听的地址。
var serverAddr = flag.String("addr", "0.0.0.0:9800", "监听地址")

// main 是程序的入口点。
// 它解析命令行标志，设置 HTTP 路由，并启动 HTTP 服务器。
func main() {
	flag.Parse() // 解析命令行标志

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html") // 处理根路径请求，返回 `static/index.html` 文件
	})
	http.Handle("/static/", http.FileServer(http.FS(static))) // 处理 `/static/` 路径下的静态文件请求
	http.HandleFunc("/proxy", proxyServer)                    // 处理 `/proxy` 路径的代理请求
	log.Printf("Video Funnel address: %s\n", *serverAddr)     // 打印服务器地址
	log.Fatal(http.ListenAndServe(*serverAddr, nil))          // 启动 HTTP 服务器并监听指定地址
}

// proxyServer 处理代理请求。
// 它解析请求中的 `link` 参数，解码 Base64 编码的 URL，并根据请求头中的 `Range` 字段处理视频流的分块请求。
func proxyServer(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm() // 解析请求表单
	if err != nil {
		log.Println(err) // 如果解析失败，记录错误并返回
		return
	}
	encoded := r.Form.Get("link")                  // 获取请求表单中的 `link` 参数
	log.Printf("proxy request url: %s\n", encoded) // 打印解码前的 URL

	urlEncode, err := base64.StdEncoding.DecodeString(encoded) // 解码 Base64 编码的 URL
	if err != nil {
		log.Fatal("解码失败:", err) // 如果解码失败，记录错误并终止程序
	}
	url := string(urlEncode) // 将解码后的 URL 转换为字符串

	if r.Header.Get("Range") == "" { // 如果请求头中没有 `Range` 字段
		resp, err := http.Get(url) // 发起 HTTP GET 请求
		if err != nil {
			w.WriteHeader(resp.StatusCode) // 如果请求失败，设置响应状态码并返回
			return
		}
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))     // 设置响应头的 `Content-Type`
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length")) // 设置响应头的 `Content-Length`
		w.Header().Set("Accept-Ranges", "bytes")                            // 设置响应头的 `Accept-Ranges`
		w.Header().Set("Connection", "keep-alive")                          // 设置响应头的 `Connection`
		w.Header().Set("ETag", resp.Header.Get("ETag"))                     // 设置响应头的 `ETag`
		w.WriteHeader(http.StatusOK)                                        // 设置响应状态码为 200 OK
		return
	}

	startPoint, endPoint := ParseRangePoint(r.Header.Get("Range")) // 解析请求头中的 `Range` 字段
	contentLength := GetContentLength(url)                         // 获取内容长度
	if endPoint == -1 {
		endPoint = contentLength - 1 // 如果结束点未指定，设置为内容长度减 1
	}

	w.Header().Set("Connection", "keep-alive")                                                          // 设置响应头的 `Connection`
	w.Header().Set("Content-Length", strconv.Itoa(endPoint-startPoint+1))                               // 设置响应头的 `Content-Length`
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startPoint, endPoint, contentLength)) // 设置响应头的 `Content-Range`
	w.WriteHeader(http.StatusPartialContent)                                                            // 设置响应状态码为 206 Partial Content

	channels := make([]chan []byte, *threadingNumber) // 创建一个通道切片，用于接收视频流数据
	for i := 0; i < *threadingNumber; i++ {
		channels[i] = make(chan []byte) // 初始化每个通道
	}

	for startPoint <= endPoint {
		for i := 0; i < *threadingNumber; i++ {
			tEndPoint := startPoint + *blockSize // 计算每个线程处理的结束点
			if tEndPoint > endPoint {
				tEndPoint = endPoint // 如果计算的结束点超过实际结束点，设置为实际结束点
			}
			go GetVideoStream(startPoint, tEndPoint, url, channels[i]) // 启动一个 goroutine 获取视频流
			startPoint = tEndPoint + 1                                 // 更新开始点
		}
		for i := 0; i < *threadingNumber; i++ {
			if t, ok := <-channels[i]; ok {
				if _, err := w.Write(t); err != nil { // 将视频流数据写入响应
					log.Println(err) // 如果写入失败，记录错误并返回
					return
				}
				log.Println("Write success") // 记录写入成功
			}
		}
	}
	log.Printf("over! %d - %d\n", startPoint, endPoint) // 记录请求处理完成
}

// ParseRangePoint 解析请求头中的 `Range` 字段，返回开始点和结束点。
func ParseRangePoint(rangeHeader string) (int, int) {
	parts := strings.Split(rangeHeader, "=") // 分割 `Range` 字段
	if len(parts) != 2 {
		return 0, -1 // 如果格式不正确，返回 0 和 -1
	}
	rangeValues := strings.Split(parts[1], "-") // 分割范围值
	if len(rangeValues) != 2 {
		return 0, -1 // 如果格式不正确，返回 0 和 -1
	}
	startPoint, _ := strconv.Atoi(rangeValues[0]) // 将开始点转换为整数
	endPoint, _ := strconv.Atoi(rangeValues[1])   // 将结束点转换为整数
	return startPoint, endPoint                   // 返回开始点和结束点
}

// GetContentLength 获取指定 URL 的内容长度。
func GetContentLength(url string) int {
	resp, err := http.Head(url) // 发起 HTTP HEAD 请求
	if err != nil {
		log.Println("获取内容长度失败:", err) // 如果请求失败，记录错误并返回 -1
		return -1
	}
	defer resp.Body.Close() // 确保响应体关闭

	contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length")) // 从响应头中获取 `Content-Length` 并转换为整数
	return contentLength                                                // 返回内容长度
}

// GetVideoStream 获取指定范围的视频流数据。
// 它根据开始点和结束点发起 HTTP GET 请求，并将响应数据写入通道。
func GetVideoStream(start, end int, url string, ch chan []byte) {
	resp, err := http.Get(fmt.Sprintf("%s?range=%d-%d", url, start, end)) // 发起 HTTP GET 请求，指定范围
	if err != nil {
		log.Println("获取视频流失败:", err) // 如果请求失败，记录错误并发送 nil 到通道
		ch <- nil
		return
	}
	defer resp.Body.Close() // 确保响应体关闭

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		log.Println("请求失败:", resp.Status) // 如果响应状态码不是 200 OK 或 206 Partial Content，记录错误并发送 nil 到通道
		ch <- nil
		return
	}

	body, err := io.ReadAll(resp.Body) // 读取响应体
	if err != nil {
		log.Println("读取响应失败:", err) // 如果读取失败，记录错误并发送 nil 到通道
		ch <- nil
		return
	}

	ch <- body // 将读取到的视频流数据发送到通道
}
