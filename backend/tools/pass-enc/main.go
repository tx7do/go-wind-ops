// pass-enc 临时冒烟工具:按前端约定(AES-128-CBC,key=iv=VITE_AES_KEY,base64)
// 加密密码。密钥经 LOGIN_AES_KEY 环境变量或 -key 参数指定(生产两端改为
// 非默认值后,调试工具须传同一密钥),缺省回落 go-utils 公开默认串(仅限开发)。
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"strings"

	"github.com/tx7do/go-utils/crypto"

	"go-wind-ops/pkg/configenv"
)

func main() {
	plain := flag.String("p", "admin", "明文密码")
	keyFlag := flag.String("key", "", "传输加密密钥(缺省读 LOGIN_AES_KEY,再回落默认串)")
	flag.Parse()

	key := configenv.LoginAESKey()
	if strings.TrimSpace(*keyFlag) != "" {
		key = []byte(*keyFlag)
	}

	enc, err := crypto.AesEncrypt([]byte(*plain), key, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(enc))
}
