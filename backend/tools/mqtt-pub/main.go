// mqtt-pub 是 MQTT 信令调试小工具:向指定主题发布任意 JSON 载荷,
// 用于手动注入指令(如 ping)验证 core/agent 链路。
//
// broker 已启用认证+ACL(EMQX 回调 core webhook):调试须以 core 账号连接,
// 且只能订发 ops/ 前缀主题;agent 身份需其本机 token 作口令。
//
// 用法:
//
//	go run ./tools/mqtt-pub -endpoint tcp://localhost:1883 \
//	  -username core -password core-pass \
//	  -topic ops/agent/<id>/command \
//	  -payload '{"request_id":"t1","agent_id":"<id>","action":"ping"}'
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	endpoint := flag.String("endpoint", "tcp://127.0.0.1:1883", "MQTT broker 地址")
	username := flag.String("username", "", "用户名")
	password := flag.String("password", "", "密码")
	clientID := flag.String("client-id", "mqtt-pub-debug", "客户端 ID")
	topic := flag.String("topic", "", "目标主题")
	payload := flag.String("payload", "{}", "JSON 载荷")
	timeout := flag.Duration("timeout", 10*time.Second, "发布超时")
	flag.Parse()

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "topic 不能为空")
		os.Exit(2)
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

	token := client.Publish(*topic, 1, false, *payload)
	if !token.WaitTimeout(*timeout) || token.Error() != nil {
		fmt.Fprintf(os.Stderr, "publish: %v\n", token.Error())
		os.Exit(1)
	}
	fmt.Printf("published %d bytes to %s\n", len(*payload), *topic)
}
