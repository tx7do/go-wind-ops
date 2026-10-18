// pass-enc 临时冒烟工具:按前端约定(AES-GCM + DefaultAESKey + base64)加密密码。
package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/tx7do/go-utils/crypto"
)

func main() {
	plain := "admin"
	if len(os.Args) > 1 {
		plain = os.Args[1]
	}
	enc, err := crypto.AesEncrypt([]byte(plain), crypto.DefaultAESKey, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(enc))
}
