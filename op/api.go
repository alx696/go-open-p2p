package op

import "encoding/json"

// TextSend 文本发送
//
// id 节点标识
//
// text 文本内容
func TextSend(id, text string) error {
	return textSend(id, text)
}

// FileSend 文件发送
//
// id 节点标识
//
// filePath 文件绝对路径
//
// 发送进度通过 Callback.OnOpFileSendProgress 获取
func FileSend(id, filePath string) error {
	return fileSend(id, filePath)
}

// ConnStateCheckSet 设置需要检查连接状态的节点标识数组
//
// 通常应该将所有联系人的标识都设置进来
//
// 连接状态通过 Callback.OnOpConnState 获取
func ConnStateCheckSet(idArrayText string) error {
	var array []string
	e := json.Unmarshal([]byte(idArrayText), &array)
	if e != nil {
		return e
	}

	connStateIdArraySet(array)

	return nil
}
