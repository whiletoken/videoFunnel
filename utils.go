package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
)

// ParseRangePoint  获取开始与结束点, 如果endpoint返回-1则表示没有指定endpoint
func ParseRangePoint(frameRange string) (startPoint, endPoint int) {
	re, _ := regexp.Compile("\\d+")
	point := re.FindAllString(frameRange, -1)

	startPoint, _ = strconv.Atoi(point[0])
	if len(point) == 1 { // 未指定结束点
		endPoint = -1
	} else {
		endPoint, _ = strconv.Atoi(point[1])
	}
	return startPoint, endPoint
}

// GetContentLength 返回请求视频的Content-Length
func GetContentLength(url string) int {
	r, _ := http.Get(url)
	ret, _ := strconv.Atoi(r.Header.Get("Content-Length"))
	return ret
}

// GetVideoStream 向真实视频服务器请求数据
func GetVideoStream(startPoint, endPoint, contentLength int, url string, ch chan []byte) {
	if startPoint > contentLength {
		close(ch)
		return
	}
	log.Printf("process %d - %d\n", startPoint, endPoint)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startPoint, endPoint))
	resp, err := client.Do(req) // 向实际服务器发送请求
	if err != nil {
		log.Fatalln(err)
	}
	defer func() { // 关闭连接
		_ = resp.Body.Close()
	}()
	data, err := ioutil.ReadAll(resp.Body)
	ch <- data
}
