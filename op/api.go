package op

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
