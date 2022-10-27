package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
)

type ResponseAnswer struct {
	Data string `json:"data"`
}

type Response struct {
	Answer []ResponseAnswer `json:"Answer"`
}

const (
	DOH = "https://doh.pub/dns-query"
)

var t *http.Transport
var c *http.Client
var ctx context.Context
var mar *madns.Resolver

func init() {
	t = &http.Transport{
		MaxIdleConns:    1,
		IdleConnTimeout: 6 * time.Second,
	}
	c = &http.Client{Transport: t}

	ctx = context.Background()
	mar = madns.DefaultResolver
}

func Txt(domian string) ([]string, error) {
	var e error
	var hr *http.Request
	hr, e = http.NewRequest(
		"GET",
		fmt.Sprintf("%s?type=16&name=%s", DOH, domian),
		nil,
	)
	if e != nil {
		return nil, e
	}
	hr.Header.Add("accept", "application/dns-json")

	var hp *http.Response
	hp, e = c.Do(hr)
	if e != nil {
		return nil, e
	}
	defer hp.Body.Close()
	var bodyBytes []byte
	bodyBytes, e = io.ReadAll(hp.Body)
	if e != nil {
		return nil, e
	}
	//log.Println("返回内容", string(bodyBytes))

	var dnsResponse Response
	e = json.Unmarshal(bodyBytes, &dnsResponse)
	if e != nil {
		return nil, e
	}

	var result []string
	ac := len(dnsResponse.Answer)
	for i, answer := range dnsResponse.Answer {
		//log.Println("返回回答", answer.Data)

		// 忽略最后一条说明信息
		if i+1 == ac {
			continue
		}

		// 去除双引号
		txt := answer.Data[1 : len(answer.Data)-1]

		result = append(result, txt)
	}

	return result, nil
}

// https://github.com/multiformats/multiaddr/blob/master/protocols/DNSADDR.md
func MaDNS(v string) ([]string, error) {
	multiAddr, multiAddrError := multiaddr.NewMultiaddr(v)
	if multiAddrError != nil {
		return nil, fmt.Errorf("多址转换错误: %w", multiAddrError)
	}

	multiAddrArray, multiAddrArrayError := mar.Resolve(ctx, multiAddr)
	if multiAddrArrayError != nil {
		return nil, fmt.Errorf("多址查询错误: %w", multiAddrArrayError)
	}

	var result []string
	for _, ma := range multiAddrArray {
		result = append(result, ma.String())
	}

	return result, nil
}
