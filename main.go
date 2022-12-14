package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go-open-p2p/op"
	"go-open-p2p/qc"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

type CallbackImpl struct {
}

// 用以捕获启动错误
var startErrorChan = make(chan error, 1)

// 用以等待彻底关闭
var stopChan = make(chan int, 1)

// 公开目录
var publicDir string

// 同步锁
var sm sync.Mutex

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

// 移动端应该设置具有权限的私有和公共文件夹路径
//
// 桌面端应该设置http服务端口
func main() {
	privateFlag := flag.String("private", "/home/m/lilu-ne/private", "private dir")
	publicFlag := flag.String("public", "/home/m/lilu-ne/public", "public dir")
	httpPortFlag := flag.Int64("http", 0, "http service port")
	flag.Parse()

	if *privateFlag == "" || *publicFlag == "" {
		log.Fatalln("没有设置文件夹")
	}

	e := os.MkdirAll(*privateFlag, os.ModePerm)
	if e != nil {
		log.Fatalln("创建私有文件夹出错", e)
	}
	publicDir = *publicFlag
	e = os.MkdirAll(publicDir, os.ModePerm)
	if e != nil {
		log.Fatalln("创建公共文件夹出错", e)
	}

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
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

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

	wsPush("OnOpState", jt)
}

func (impl CallbackImpl) OnOpMDNSPeer(id string) {
	log.Println("回调MDNS发现节点", id)

	wsPush("OnOpMDNSPeer", id)
}

func (impl CallbackImpl) OnOpConnState(id string, isConn bool) {
	//log.Println("回调节点连接状态变化", id, isConn)

	m := map[string]interface{}{"id": id, "conn": isConn}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println("节点连接状态变化数据转JSON出错", e)
	} else {
		wsPush("OnOpConnState", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpTextSendError(uuid, et string) {
	log.Println("回调文本发送出错", uuid, et)

	m := map[string]interface{}{
		"uuid": uuid,
		"et":   et,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpTextSendError", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpTextSendDone(uuid string) {
	log.Println("回调文本发送完成本", uuid)

	m := map[string]interface{}{
		"uuid": uuid,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpTextSendDone", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpTextReceiveDone(id, text string) {
	log.Println("回调文本接收完毕", id, text)

	m := map[string]interface{}{"id": id, "text": text}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println("文本接收完毕数据转JSON出错", e)
	} else {
		wsPush("OnOpTextReceiveDone", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileSendError(uuid, et string) {
	log.Println("回调文件发送出错", uuid, et)

	m := map[string]interface{}{
		"uuid": uuid,
		"et":   et,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileSendError", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileSendProgress(uuid string, fileSize, sendSize int64) {
	log.Println("回调文件发送进度", uuid, fileSize, sendSize)

	m := map[string]interface{}{
		"uuid":     uuid,
		"fileSize": fileSize,
		"sendSize": sendSize,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileSendProgress", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileSendDone(uuid, fileHash string) {
	log.Println("回调文件发送完成", uuid, fileHash)

	m := map[string]interface{}{
		"uuid":     uuid,
		"fileHash": fileHash,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileSendDone", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileReceiveStart(id, fileHash, fileName, uuid string, fileSize int64) {
	log.Println("回调文件接收开始", id, fileHash, fileName, uuid, fileSize)

	m := map[string]interface{}{
		"id":       id,
		"fileHash": fileHash,
		"fileName": fileName,
		"uuid":     uuid,
		"fileSize": fileSize,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileReceiveStart", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileReceiveError(uuid, et string) {
	log.Println("回调文件接收错误", uuid, et)

	m := map[string]interface{}{
		"uuid": uuid,
		"et":   et,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileReceiveError", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileReceiveProgress(uuid string, fileSize, receiveSize int64) {
	log.Println("回调文件接收进度", uuid, fileSize, receiveSize)

	m := map[string]interface{}{
		"uuid":        uuid,
		"fileSize":    fileSize,
		"receiveSize": receiveSize,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileReceiveProgress", string(jsonBytes))
	}
}

func (impl CallbackImpl) OnOpFileReceiveDone(uuid, filePath string) {
	log.Println("回调文件接收完毕", uuid, filePath)

	m := map[string]interface{}{
		"uuid":     uuid,
		"filePath": filePath,
	}
	jsonBytes, e := json.Marshal(m)
	if e != nil {
		log.Println(e)
	} else {
		wsPush("OnOpFileReceiveDone", string(jsonBytes))
	}
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
	m := map[string]interface{}{"c": c, "t": t}
	jsonBytes, _ := json.Marshal(m)
	//log.Println("ws推送", string(jsonBytes))

	sm.Lock()
	for _, conn := range wsConnMap {
		//log.Println("推送ws", id)

		_ = conn.WriteMessage(websocket.TextMessage, jsonBytes)
		//if e != nil {
		//	log.Println("推送ws失败", id, c, e)
		//} else {
		//	log.Println("推送ws成功", id, c)
		//}
	}
	sm.Unlock()
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
		case "/bootstrap":
			httpHandlerBootstrapSet(ctx)
		case "/feed":
			httpHandlerFeed(ctx)
		case "/send/text":
			httpHandlerTextSend(ctx)
		case "/send/file":
			httpHandlerFileSend(ctx)
		case "/conn/check":
			httpHandlerConnStateCheckSet(ctx)
		case "/qrcode":
			httpHandlerQrcode(ctx)
		case "/check/id":
			httpHandlerCheckId(ctx)
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

func httpHandlerBootstrapSet(ctx *fasthttp.RequestCtx) {
	reqArray := string(ctx.FormValue("array"))

	if reqArray == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	e := op.BootstrapSet(reqArray)
	if e != nil {
		log.Println(e)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}

	return
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
					sm.Lock()
					e := conn.WriteMessage(websocket.PingMessage, nil)
					sm.Unlock()
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
		return
	}

	op.TextSend(reqUUID, reqID, reqText)
}

func httpHandlerFileSend(ctx *fasthttp.RequestCtx) {
	reqUUID := string(ctx.FormValue("uuid"))
	reqID := string(ctx.FormValue("id"))
	reqPath := string(ctx.FormValue("path"))

	if reqUUID == "" || reqID == "" || reqPath == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	op.FileSend(reqUUID, reqID, reqPath)
}

func httpHandlerConnStateCheckSet(ctx *fasthttp.RequestCtx) {
	reqIdArray := string(ctx.FormValue("id_array"))

	if reqIdArray == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	e := op.ConnStateCheckSet(reqIdArray)
	if e != nil {
		log.Println(e)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}

	return
}

func httpHandlerQrcode(ctx *fasthttp.RequestCtx) {
	reqText := string(ctx.FormValue("text"))

	if reqText == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	fileName := "qrcode-my.jpg"
	imgPath := filepath.Join(publicDir, fileName)
	e := qc.Encode(imgPath, reqText, 256, 256)
	if e != nil {
		log.Println(e)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	fileBytes, e := ioutil.ReadFile(imgPath)
	if e != nil {
		log.Println("读取文件错误:", e)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	ctx.Response.Header.Set("Content-Disposition", fmt.Sprint("attachment;filename=", url.QueryEscape(fileName)))
	ctx.SetBody(fileBytes)
}

func httpHandlerCheckId(ctx *fasthttp.RequestCtx) {
	reqId := string(ctx.FormValue("id"))

	if reqId == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	ok := op.IdOk(reqId)
	if ok {
		ctx.SetStatusCode(fasthttp.StatusOK)
	} else {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}

	return
}
