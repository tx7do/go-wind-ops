// mqtt-pub 是 MQTT 信令调试小工具:向指定主题发布任意 JSON 载荷,
// 用于手动注入指令(如 ping)验证 core/agent 链路。
//
// broker 已启用认证+ACL(EMQX 回调 core webhook):调试须以 core 账号连接,
// 且只能订发 ops/ 前缀主题;agent 身份需其本机 token 作口令。
// agent 配置了 COMMAND_SIGN_KEY 后强制校验指令签名:注入指令须以
// -sign-key 传入同一密钥(工具会补 issued_at 并签名),否则被拒收。
//
// 用法:
//
//	go run ./tools/mqtt-pub -endpoint tcp://localhost:1883 \
//	  -username core -password core-pass \
//	  -topic ops/agent/<id>/command \
//	  -payload '{"request_id":"t1","agent_id":"<id>","action":"ping"}' \
//	  -sign-key <COMMAND_SIGN_KEY>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"go-wind-ops/pkg/signal"
)

func main() {
	endpoint := flag.String("endpoint", "tcp://127.0.0.1:1883", "MQTT broker 地址")
	username := flag.String("username", "", "用户名")
	password := flag.String("password", os.Getenv("MQTT_CORE_PASSWORD"), "密码(缺省读 MQTT_CORE_PASSWORD 环境变量)")
	clientID := flag.String("client-id", "mqtt-pub-debug", "客户端 ID")
	topic := flag.String("topic", "", "目标主题")
	payload := flag.String("payload", "{}", "JSON 载荷")
	signKey := flag.String("sign-key", "", "指令签名密钥(COMMAND_SIGN_KEY 配置后必填)")
	timeout := flag.Duration("timeout", 10*time.Second, "发布超时")
	flag.Parse()

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "topic 不能为空")
		os.Exit(2)
	}

	out := []byte(*payload)
	if *signKey != "" {
		var env signal.CommandEnvelope
		if err := json.Unmarshal(out, &env); err != nil {
			fmt.Fprintf(os.Stderr, "payload 不是合法指令信封 JSON: %v\n", err)
			os.Exit(2)
		}
		if env.IssuedAt.IsZero() {
			env.IssuedAt = time.Now()
		}
		signal.SignEnvelope(&env, *signKey)
		encoded, err := json.Marshal(env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal signed envelope: %v\n", err)
			os.Exit(1)
		}
		out = encoded
	}

	opts := paho.NewClientOptions().
		AddBroker(*endpoint).
		SetClientID(*clientID).
		SetAutoReconnect(false)
	if *username != "" {
		opts = opts.SetUsername(*username).SetPassword(*password)
	}

	client := paho.NewClient(opts)
	if token := client.Connect(); token.WaitTimeout(*timeout) && token.Error() != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", token.Error())
		os.Exit(1)
	}
	defer client.Disconnect(250)

	token := client.Publish(*topic, 1, false, out)
	if !token.WaitTimeout(*timeout) || token.Error() != nil {
		fmt.Fprintf(os.Stderr, "publish: %v\n", token.Error())
		os.Exit(1)
	}
	fmt.Printf("published %d bytes to %s\n", len(out), *topic)
}
