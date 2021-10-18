package op

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"io/ioutil"
	"os"
)

// 获取密钥(没有时生成, 存在时加载)
func getPrivateKey(privateKeyPath string) (*crypto.PrivKey, error) {
	var privateKey crypto.PrivKey
	var privateKeyBytes []byte
	_, e := os.Stat(privateKeyPath)
	if os.IsNotExist(e) {
		privateKey, _, e = crypto.GenerateKeyPair(
			crypto.Ed25519, // Select your key type. Ed25519 are nice short
			-1,             // Select key length when possible (i.e. RSA).
		)
		if e != nil {
			return nil, e
		}
		privateKeyBytes, e = crypto.MarshalPrivateKey(privateKey)
		if e != nil {
			return nil, e
		}
		e = ioutil.WriteFile(privateKeyPath, privateKeyBytes, os.ModePerm)
		if e != nil {
			return nil, e
		}
	} else {
		privateKeyBytes, e = ioutil.ReadFile(privateKeyPath)
		if e != nil {
			return nil, e
		}
		privateKey, e = crypto.UnmarshalPrivateKey(privateKeyBytes)
		if e != nil {
			return nil, e
		}
	}
	return &privateKey, nil
}
