package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fasthttp/websocket"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"go-open-p2p/op"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type CallbackImpl struct {
}

type WsMessage struct {
	C string `json:"c"`
	T string `json:"t"`
}

// 用以捕获启动错误
var startErrorChan = make(chan error, 1)

// 用以等待彻底关闭
var stopChan = make(chan int, 1)

// 应用目录
var appDir string

// 同步锁
var sm sync.RWMutex

var wsUpgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	//跨域
	CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
		return true
	},
}

var wsConnMap = make(map[string]*websocket.Conn)

// 节点ID
var opID string

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

	sm.Lock()
	opID = id
	sm.Unlock()

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

	wsPush("state", jt)
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

// 更新WebSocket连接
//
// conn 设为nil表示删除并关闭连接
func wsConnUpdate(id string, conn *websocket.Conn) {
	sm.Lock()
	oldConn, connExists := wsConnMap[id]
	if connExists {
		log.Println("关闭WebSocket连接", id)
		_ = oldConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "服务主动关闭"))
		_ = oldConn.Close()
		delete(wsConnMap, id)
	}
	if conn != nil {
		wsConnMap[id] = conn
	}
	sm.Unlock()
}

// 推送给WebSocket
func wsPush(c, t string) {
	wm := WsMessage{C: c, T: t}
	jsonBytes, _ := json.Marshal(wm)

	sm.RLock()
	for id, conn := range wsConnMap {
		log.Println("推送ws", id)

		e := conn.WriteMessage(websocket.TextMessage, jsonBytes)
		if e != nil {
			log.Println("推送ws失败", id, c, e)
		} else {
			log.Println("推送ws成功", id, c)
		}
	}
	sm.RUnlock()
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
		case "/feed":
			httpHandlerFeed(ctx)
		case "/send/text":
			httpHandlerTextSend(ctx)
		case "/send/file":
			httpHandlerFileSend(ctx)
		case "/conn/check":
			httpHandlerConnStateCheckSet(ctx)
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

// 检测节点是否已经启动
func httpHandlerRoot(ctx *fasthttp.RequestCtx) {
	if opID != "" {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody([]byte(opID))
	} else {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
	}
}

// 订阅
func httpHandlerFeed(ctx *fasthttp.RequestCtx) {
	e := wsUpgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		defer conn.Close()

		// 读取请求
		messageType, messageBytes, e := conn.ReadMessage()
		if e != nil || messageType != websocket.TextMessage {
			log.Println("订阅请求必须是文本: ", e)
			return
		}
		requestText := string(messageBytes)
		log.Println("订阅请求: ", requestText)
		clientID := uuid.New().String()

		// 保持连接, 检测连接断开
		var closeChan = make(chan error, 1)
		ticker := time.NewTicker(time.Second)
		go func() {
			for {
				select {
				case <-ticker.C:
					e := conn.WriteMessage(websocket.PingMessage, nil)
					if e != nil {
						closeChan <- e
						return
					}
					//// 发送测试
					//e = conn.WriteMessage(websocket.TextMessage, []byte(time.Now().String()))
					//log.Println("发送测试是否出错", e)
				}
			}
		}()

		// 缓存连接, 用于批量发送回调
		wsConnUpdate(clientID, conn)

		closeError := <-closeChan
		log.Println("订阅连接断开(ping报错)", clientID, closeError)

		// 移除连接
		wsConnUpdate(clientID, nil)
	})
	if e != nil {
		log.Println(e)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func httpHandlerTextSend(ctx *fasthttp.RequestCtx) {
	reqUUID := string(ctx.FormValue("uuid"))
	reqID := string(ctx.FormValue("id"))
	reqText := string(ctx.FormValue("text"))

	if reqUUID == "" || reqID == "" || reqText == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}

	op.TextSend(reqUUID, reqID, reqText)
}

func httpHandlerFileSend(ctx *fasthttp.RequestCtx) {
	reqUUID := string(ctx.FormValue("uuid"))
	reqID := string(ctx.FormValue("id"))
	reqPath := string(ctx.FormValue("path"))

	if reqUUID == "" || reqID == "" || reqPath == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}

	op.FileSend(reqUUID, reqID, reqPath)
}

func httpHandlerConnStateCheckSet(ctx *fasthttp.RequestCtx) {
	reqJSON := string(ctx.FormValue("json"))

	e := op.ConnStateCheckSet(reqJSON)
	if e != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}
}
