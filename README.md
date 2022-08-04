通过多线程请求并缓存数据来加速视频播放

![Snipaste_2022-08-04_08-43-48](https://blog.dongliu.site/media/image/Snipaste_2022-08-04_08-43-48-16595853521662.png)

### 1. 开始使用

自己编译或者到`release`页面下载对应系统的可执行文件，然后直接运行就可以了

```bash
./videoFunnel
```

如果要修改默认参数可以输入`--help`参数查看帮助

```bash
Usage of ./videoFunnel:
  -addr string
        listen address (default "localhost:9800")
  -p int
        threading number (default 4)
  -s int
        block size (default 1048576)
```

程序成功运行之后会启动一个web服务，在输入框内输入需要代理的视频地址，在点击`Go`之后会自动将代理之后的地址复制到剪切板，如果可以是能被浏览器解码的视频可以直接通过下面的HTML 5的视频控件播放，不能解码的视频可以通过`vlc`的串流来播放。

### 2. 技术细节

**实现原理**

当客户端直接向服务器请求视频播放的时候，一般只会采用单线程请求数据当网络情况的时候速度较慢，所以我就想是不是可以通过多线程请求并将数据缓存下来加速视频播放，我先用了一下`aria2`的多线程顺序下载，确实可提升视频播放速度但是无法快进，所以就决定自己写一个。

**请求部分数据**

当你点击视频中未缓存的一点时浏览器并不会一直缓存到那个地方再继续播放，而是发起`range`请求中间部分数据，具体的请求流程可以参考`MDN`的[Range requests](https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests)。

**线程同步**

在通过多线程请求的时候需要保证请求的数据按照一定的顺序发送给客户端，我这里使用的是`go`的`channel`，有多少`goroutines`就建立多大的`channel`数组，每一个`goroutines`写入其对应的`channel`，然后按照数组的顺序读取`channel`就可以了。
