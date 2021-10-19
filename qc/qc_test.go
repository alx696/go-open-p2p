package qc_test

import (
	"io/ioutil"
	"log"
	"testing"

	"go-open-p2p/qc"
)

func TestEncode(t *testing.T) {
	// http://xm.lilu.red:4003/ipfs/Qmez9NzFebCJyogZVDWv4M7Pbdhe8jeVyCcUo1h2Gu729t/云南虫谷第01集.mp4

	e := qc.Encode(
		"/home/gs/图片/qrcode.jpg",
		"https://cast.lilu.red/?id=12D3KooWAHdmZpVXsQyiyaq7PzZ9FnLnPpYKV1xmzFwbhYVWJ53L&password=633083&name=generic_x86+sdk_google_atv_x86",
		512, 512,
	)
	if e != nil {
		log.Fatalln(e)
	}
}

func TestDecode(t *testing.T) {
	data, e := ioutil.ReadFile("/home/gs/图片/qrcode.jpg")
	if e != nil {
		log.Fatalln(e)
	}

	txt, e := qc.DecodeBytes(data)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(txt)
}
