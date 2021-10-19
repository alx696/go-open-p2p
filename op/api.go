package op

import "encoding/json"

// TextSend 文本发送
func TextSend(id, text string) error {
	return textSend(id, text)
}

// FileSend 文件发送
//
// 发送进度通过 Callback.OnOpFileSendProgress 获取
func FileSend(id, filePath string) error {
	return fileSend(id, filePath)
}

// ConnStateCheckSet 设置需要检查连接状态的节点ID数组
func ConnStateCheckSet(idArrayText string) error {
	var array []string
	e := json.Unmarshal([]byte(idArrayText), &array)
	if e != nil {
		return e
	}

	connStateIdArraySet(array)

	return nil
}
