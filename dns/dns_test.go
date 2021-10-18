package dns_test

import (
	"go-open-p2p/dns"
	"log"
	"testing"
)

func TestTxt(t *testing.T) {
	log.Println(dns.Txt("bootstrap.ipfs.lilu.red"))
}

func TestMaDNS(t *testing.T) {
	log.Println(dns.MaDNS("/dnsaddr/bootstrap.libp2p.io"))
}
