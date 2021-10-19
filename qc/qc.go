package qc

import (
	"bytes"
	"image"
	"image/jpeg"
	"log"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

var qrcodeReader gozxing.Reader
var qrcodeWriter gozxing.Writer

func init() {
	qrcodeReader = qrcode.NewQRCodeReader()
	qrcodeWriter = qrcode.NewQRCodeWriter()
}

// Encode 二维码编码
func Encode(path, text string, width int, height int) error {
	log.Println("二维码编码", path, text, width, height)

	hints := make(map[gozxing.EncodeHintType]interface{})
	hints[gozxing.EncodeHintType_MARGIN] = 0

	bm, e := qrcodeWriter.Encode(text, gozxing.BarcodeFormat_QR_CODE, width, height, hints)
	if e != nil {
		return e
	}

	var file *os.File
	file, e = os.Create(path)
	if e != nil {
		return e
	}
	defer file.Close()
	e = jpeg.Encode(file, bm, nil)
	if e != nil {
		return e
	}

	return nil
}

// DecodeBytes 二维码解码图片字节
func DecodeBytes(data []byte) (string, error) {
	img, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return "", e
	}

	// prepare BinaryBitmap
	bmp, e := gozxing.NewBinaryBitmapFromImage(img)
	if e != nil {
		return "", e
	}

	// decode image
	result, e := qrcodeReader.Decode(bmp, nil)
	if e != nil {
		return "", e
	}

	return result.GetText(), nil
}

// DecodeYUV 二维码解码YUV
func DecodeYUV(yuvData []byte, dataWidth int, dataHeight int) (string, error) {
	source, e := gozxing.NewPlanarYUVLuminanceSource(yuvData, dataWidth, dataHeight, 0, 0, dataWidth, dataHeight, false)
	if e != nil {
		return "", e
	}

	var bm *gozxing.BinaryBitmap
	bm, e = gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(source))
	if e != nil {
		return "", e
	}

	// decode image
	var result *gozxing.Result
	result, e = qrcodeReader.Decode(bm, nil)
	if e != nil {
		return "", e
	}

	return result.GetText(), nil
}
