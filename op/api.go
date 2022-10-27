package op

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TODO 发送需要支持取消

// 检查ID是否有效
func IdOk(id string) bool {
	_, e := peer.Decode(id)
	return e == nil
}

// 设置引导
func BootstrapSet(arrayText string) error {
	var array []string
	e := json.Unmarshal([]byte(arrayText), &array)
	if e != nil {
		return e
	}

	for _, v := range array {
		go connectBootstrap(globalContext, globalHost, v)
	}

	return nil
}

// TextSend 文本发送
//
// uuid 唯一标识, 用于跟踪状态
//
// id 节点标识
//
// text 文本内容
//
// 发送错误通过 Callback.OnOpTextSendError 获取
//
// 发送完成通过 Callback.OnOpTextSendDone 获取
func TextSend(uuid, id, text string) {
	go textSend(uuid, id, text)
}

// FileSend 文件发送
//
// uuid 唯一标识, 用于跟踪状态
//
// id 节点标识
//
// filePath 文件绝对路径
//
// 发送错误通过 Callback.OnOpFileSendError 获取
//
// 发送进度通过 Callback.OnOpFileSendProgress 获取
//
// 发送完成通过 Callback.OnOpFileSendDone 获取
func FileSend(uuid, id, filePath string) {
	go fileSend(uuid, id, filePath)
}

// ConnStateCheckSet 设置需要检查连接状态的节点标识数组
//
// 通常应该将所有联系人的标识都设置进来
//
// 连接状态通过 Callback.OnOpConnState 获取
func ConnStateCheckSet(arrayText string) error {
	var array []string
	e := json.Unmarshal([]byte(arrayText), &array)
	if e != nil {
		return e
	}

	connStateIdArraySet(array)

	return nil
}
