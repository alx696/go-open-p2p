package main

import (
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"go-open-p2p/op"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
)

type CallbackImpl struct {
}

// 用以捕获启动错误
var startErrorChan = make(chan error, 1)

// 用以等待彻底关闭
var stopChan = make(chan int, 1)

var appDir string

func main() {
	privateFlag := flag.String("private", "", "private dir")
	publicFlag := flag.String("public", "", "public dir")
	nameFlag := flag.String("name", "", "my name")
	httpPortFlag := flag.Int64("http", 0, "http service port")
	flag.Parse()

	if *privateFlag == "" || *publicFlag == "" || *nameFlag == "" {
		log.Fatalln("没有设置参数")
	}

	// 获取应用目录
	var e error
	appDir, e = filepath.Abs(filepath.Dir(os.Args[0]))
	if e != nil {
		log.Fatalln(e)
	}
	log.Println("应用目录:", appDir)

	go func() {
		e := op.Start(*privateFlag, *publicFlag, CallbackImpl{})
		if e != nil {
			startErrorChan <- e
		}
	}()

	httpPort := *httpPortFlag
	if httpPort != 0 {
		go func() {
			log.Println("开始启动HTTP服务:", httpPort)
			e := startHTTP(httpPort)
			if e != nil {
				startErrorChan <- e
			}
		}()
	}

	// 关注系统信号
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// 等待
	select {
	case s := <-signalChan:
		log.Println("收到系统信号", s.String())
		op.Stop()
	case e := <-startErrorChan:
		log.Println("启动出错", e)
		stopChan <- 1
	}

	<-stopChan
	log.Println("程序已经关闭")
}

func (impl CallbackImpl) OnOpStart(id string, addrArray string) {
	log.Println("回调启动", id, addrArray)

	//go func() {
	//	<-time.After(time.Second * 6)
	//
	//	log.Println("测试发送文本")
	//	op.TextSend(uuid.New().String(), "12D3KooWS7mTsSGrngHP1YhNsiFZWYHRGpxgmfXoFcUnaU8KHbJt", "你好")
	//
	//	log.Println("测试发送文件")
	//	op.FileSend(uuid.New().String(), "12D3KooWS7mTsSGrngHP1YhNsiFZWYHRGpxgmfXoFcUnaU8KHbJt", "/home/gs/下载/timescale-br.zip")
	//
	//	//var connStateIdArray []string
	//	//connStateIdArray = append(connStateIdArray, "12D3KooWS7mTsSGrngHP1YhNsiFZWYHRGpxgmfXoFcUnaU8KHbJt")
	//	//jsonBytes, _ := json.Marshal(connStateIdArray)
	//	//e := op.ConnStateCheckSet(string(jsonBytes))
	//	//if e != nil {
	//	//	log.Println("设置状态检查ID数组出错", e)
	//	//} else {
	//	//	log.Println("设置状态检查ID数组成功")
	//	//}
	//}()
}

func (impl CallbackImpl) OnOpStop() {
	log.Println("回调停止")
	stopChan <- 1
}

func (impl CallbackImpl) OnOpState(jt string) {
	//log.Println("回调状态", jt)
}

func (impl CallbackImpl) OnOpMDNSPeer(id string) {
	log.Println("回调MDNS发现节点", id)
}

func (impl CallbackImpl) OnOpConnState(id string, isConn bool) {
	log.Println("回调节点连接状态变化", id, isConn)
}

func (impl CallbackImpl) OnOpTextSendError(uuid, et string) {
	log.Println("回调文本发送出错", uuid, et)
}

func (impl CallbackImpl) OnOpTextSendDone(uuid string) {
	log.Println("回调文本发送完成本", uuid)
}

func (impl CallbackImpl) OnOpTextReceiveDone(id, text string) {
	log.Println("回调文本接收完毕", id, text)
}

func (impl CallbackImpl) OnOpFileSendError(uuid, et string) {
	log.Println("回调文件发送出错", uuid, et)
}

func (impl CallbackImpl) OnOpFileSendProgress(uuid string, fileSize, sendSize int64) {
	log.Println("回调文件发送进度", uuid, fileSize, sendSize)
}

func (impl CallbackImpl) OnOpFileSendDone(uuid, fileHash string) {
	log.Println("回调文件发送完成", uuid, fileHash)
}

func (impl CallbackImpl) OnOpFileReceiveStart(id, fileHash, fileName, uuid string, fileSize int64) {
	log.Println("回调文件接收开始", id, fileHash, fileName, uuid, fileSize)
}

func (impl CallbackImpl) OnOpFileReceiveError(uuid, et string) {
	log.Println("回调文件接收错误", uuid, et)
}

func (impl CallbackImpl) OnOpFileReceiveProgress(uuid string, fileSize, receiveSize int64) {
	log.Println("回调文件接收进度", uuid, fileSize, receiveSize)
}

func (impl CallbackImpl) OnOpFileReceiveDone(uuid, filePath string) {
	log.Println("回调文件接收完毕", uuid, filePath)
}

func startHTTP(p int64) error {
	requestHandler := func(ctx *fasthttp.RequestCtx) {
		//CORS
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, OPTIONS, POST, PUT, DELETE")
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
		ctx.Response.Header.Set("Access-Control-Expose-Headers", "x-name, x-size")

		//防止客户端发送OPTIONS时报错
		if string(ctx.Method()) == "OPTIONS" {
			return
		}

		//PATH
		// log.Println(string(ctx.Path()), string(ctx.Method()))
		switch string(ctx.Path()) {
		case "/":
			httpHandlerRoot(ctx)
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}
	fhServer := &fasthttp.Server{
		Name: "Open P2P HTTP Service",
		// Other Server settings may be set here.
		MaxRequestBodySize: 1024 * 1024 * 18,
		Handler:            requestHandler,
	}
	return fhServer.ListenAndServe(fmt.Sprint(":", strconv.FormatInt(p, 10)))
}

func httpHandlerRoot(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody([]byte("运行"))
}
