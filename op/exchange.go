package op

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func initExchange(h host.Host) {
	log.Println("初始化交换")
	h.SetStreamHandler(protocolText, textStreamHandler)
	h.SetStreamHandler(protocolFile, fileStreamHandler)
}

// 文本处理
func textStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("文本处理, 对方ID:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 读取
	requestBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		log.Println("文本处理, 读取对方发来内容出错:", e)
		return
	}
	requestText := string(*requestBytes)
	log.Println("文本处理, 对方发来内容:", requestText)

	// 通知收到
	globalCallback.OnOpText(requestText)

	// 回复1
	responseBytes := []byte("成功")
	e = writeTextToReadWriter(rw, &responseBytes)
	if e != nil {
		log.Println("文本处理, 回复对方成功时出错:", e)
	}
}

// 文本发送
func textSend(id, text string) error {
	s, e := createStream(globalContext, globalHost, id, protocolText)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 写入
	data := []byte(text)
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		return e
	}

	// 接收
	resultBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		return e
	}
	resultText := string(*resultBytes)

	// 检查异常状态
	if resultText != "成功" {
		return fmt.Errorf("发送文本失败:%s", resultText)
	}

	return nil
}

// 文件处理
func fileStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("文件处理, 对方ID:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 读取文件哈希
	requestBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		log.Println("文件处理, 读取对方文件哈希出错:", e)
		return
	}
	fileHash := string(*requestBytes)
	log.Println("文件处理, 对方发来文件哈希:", fileHash)

	// 读取文件大小
	requestBytes, e = readTextFromReadWriter(rw)
	if e != nil {
		log.Println("文件处理, 读取对方文件大小出错:", e)
		return
	}
	fileSizeText := string(*requestBytes)
	log.Println("文件处理, 对方发来文件大小:", fileSizeText)
	fileSize, e := strconv.ParseInt(fileSizeText, 10, 64)
	if e != nil {
		log.Println("文件处理, 转换对方文件大小出错:", e)
		return
	}

	// 读取文件名称
	requestBytes, e = readTextFromReadWriter(rw)
	if e != nil {
		log.Println("文件处理, 读取对方文件名称出错:", e)
		return
	}
	fileName := string(*requestBytes)
	log.Println("文件处理, 对方发来文件名称:", fileName)

	// 准备文件路径
	fileCacheDir := filepath.Join(globalPublicDirectory, ".CACHE")
	e = os.MkdirAll(fileCacheDir, os.ModePerm)
	if e != nil {
		log.Println("文件处理, 创建缓存文件夹出错:", e)
		return
	}
	fileCachePath := filepath.Join(fileCacheDir, fileHash)

	// 根据文件哈希确定已经接收大小
	var finishSize int64
	fileInfo, e := os.Stat(fileCachePath)
	if e == nil {
		finishSize = fileInfo.Size()
	}
	log.Println("文件处理, 已经接收大小:", fileCachePath, finishSize)
	// 写入已经接收大小
	data := []byte(strconv.FormatInt(finishSize, 10))
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		log.Println("文件处理, 写入已经接收大小出错:", e)
		return
	}

	// 开始接收文件
	f, _ := os.OpenFile(fileCachePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	defer f.Close()
	var doneSum int64 //完成长度
	buf := make([]byte, 1048576)
	for {
		var rn int
		rn, e = rw.Read(buf)
		if e != nil {
			if e == io.EOF {
				log.Println("文件处理: 读取完毕")
			} else {
				log.Println("文件处理: 读取数据出错", e)
				// 告知接收错误
				globalCallback.OnOpFileReceiveError(remotePeerID.Pretty(), fileCachePath, e.Error())
				return
			}
		}

		var wn int
		if rn > 0 {
			wn, e = f.Write(buf[0:rn])
			if e != nil {
				log.Println("文件处理: 保存数据出错", e)
				// 告知接收错误
				globalCallback.OnOpFileReceiveError(remotePeerID.Pretty(), fileCachePath, e.Error())
				return
			}
		}

		// 累加完成长度
		doneSum += int64(wn)
		// 计算百分比
		percentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(doneSum)/float64(fileSize)), 64)
		// 告知接收进度
		globalCallback.OnOpFileReceiveProgress(remotePeerID.Pretty(), fileCachePath, percentage)

		// 判断是否完成
		if finishSize+doneSum == fileSize {
			break
		}
	}

	// 移动缓存文件为正式文件
	filePath := filepath.Join(globalPublicDirectory, fileName)
	_, e = os.Stat(filePath)
	if e == nil {
		filePath = filepath.Join(globalPublicDirectory, fmt.Sprintf("[%d]%s", time.Now().Nanosecond(), fileName))
	}
	e = os.Rename(fileCachePath, filePath)
	if e != nil {
		log.Println("文件处理, 移动缓存文件为正式文件出错:", e)
		// 告知接收错误
		globalCallback.OnOpFileReceiveError(remotePeerID.Pretty(), fileCachePath, e.Error())
		return
	}

	// 告知接收完成
	globalCallback.OnOpFileReceiveDone(remotePeerID.Pretty(), filePath)

	// 回复1
	responseBytes := []byte("成功")
	e = writeTextToReadWriter(rw, &responseBytes)
	if e != nil {
		log.Println("文件处理, 回复对方成功时出错:", e)
	}
}

// 文件发送
func fileSend(id, filePath string) error {
	s, e := createStream(globalContext, globalHost, id, protocolFile)
	if e != nil {
		return e
	}
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 获取文件信息
	fileInfo, e := os.Stat(filePath)
	if e != nil {
		return e
	}
	fileSize := fileInfo.Size()
	fileName := fileInfo.Name()

	// 获取文件哈希
	f1, e := os.Open(filePath)
	if e != nil {
		return e
	}
	shaHash := sha256.New()
	if _, e := io.Copy(shaHash, f1); e != nil {
		return e
	}
	fileHash := fmt.Sprintf("%x", shaHash.Sum(nil))
	e = f1.Close()
	if e != nil {
		return e
	}

	// 写入文件哈希
	data := []byte(fileHash)
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		return e
	}

	// 写入文件大小
	data = []byte(strconv.FormatInt(fileSize, 10))
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		return e
	}

	// 写入文件名称
	data = []byte(fileName)
	e = writeTextToReadWriter(rw, &data)
	if e != nil {
		return e
	}

	// 接收已经发送大小
	resultBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		return e
	}
	resultText := string(*resultBytes)
	sendSize, e := strconv.ParseInt(resultText, 10, 64)
	if e != nil {
		return e
	}
	log.Println("文件发送, 已经完成大小", sendSize)

	// 写入文件数据
	f2, e := os.Open(filePath)
	if e != nil {
		return e
	}
	defer f2.Close()

	// 移动到续传位置
	_, e = f2.Seek(sendSize, 0)
	if e != nil {
		return e
	}

	var doneSum int64 //完成长度
	buf := make([]byte, 1048576)
	for {
		rn, e := f2.Read(buf)
		if e != nil {
			if e == io.EOF {
				log.Println("发送文件数据读取完毕")
			} else {
				log.Println("发送文件读取数据出错", e)
				return e
			}
		}

		var wn int
		if rn > 0 {
			wn, e = rw.Write(buf[0:rn])
			if e != nil {
				return e
			}
		}

		// 累加完成长度
		doneSum += int64(wn)
		percentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(doneSum)/float64(fileSize)), 64)

		// 通知发送进度
		globalCallback.OnOpFileSendProgress(id, filePath, percentage)

		if sendSize+doneSum == fileSize {
			break
		}
	}
	e = rw.Flush()
	if e != nil {
		return e
	}

	// 接收结果
	resultBytes, e = readTextFromReadWriter(rw)
	if e != nil {
		return e
	}
	resultText = string(*resultBytes)

	// 检查异常状态
	if resultText != "成功" {
		return fmt.Errorf("发送文件失败:%s", resultText)
	}

	//// 通知发送完毕
	//globalCallback.OnOpFileSendDone(id, filePath)

	return nil
}
