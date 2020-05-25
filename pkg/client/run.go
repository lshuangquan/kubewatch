/*
Copyright 2016 Skippbox, Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"flag"
	"fmt"
	websocket2 "github.com/gorilla/websocket"
	"kubewatch/config"
	"kubewatch/pkg/controller"
	"kubewatch/pkg/handlers"
	"kubewatch/pkg/handlers/flock"
	"kubewatch/pkg/handlers/hipchat"
	"kubewatch/pkg/handlers/mattermost"
	"kubewatch/pkg/handlers/msteam"
	"kubewatch/pkg/handlers/redis"
	"kubewatch/pkg/handlers/slack"
	"kubewatch/pkg/handlers/webhook"
	"kubewatch/pkg/handlers/websocket"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// Run runs the event loop processing with given handler
func Run(conf *config.Config) {
	gentleExit()
	var eventHandler = ParseEventHandler(conf)
	if conf.Handler.Websocket.Url != "" {
		go controller.Start(conf, eventHandler)
		startWebSocket(conf)
	}else {
		controller.Start(conf, eventHandler)
	}
}

// ParseEventHandler returns the respective handler object specified in the config file.
func ParseEventHandler(conf *config.Config) handlers.Handler {
	var eventHandler handlers.Handler
	switch {
	case len(conf.Handler.Slack.Channel) > 0 || len(conf.Handler.Slack.Token) > 0:
		eventHandler = new(slack.Slack)
	case len(conf.Handler.Hipchat.Room) > 0 || len(conf.Handler.Hipchat.Token) > 0:
		eventHandler = new(hipchat.Hipchat)
	case len(conf.Handler.Mattermost.Channel) > 0 || len(conf.Handler.Mattermost.Url) > 0:
		eventHandler = new(mattermost.Mattermost)
	case len(conf.Handler.Flock.Url) > 0:
		eventHandler = new(flock.Flock)
	case len(conf.Handler.Webhook.Url) > 0:
		eventHandler = new(webhook.Webhook)
	case len(conf.Handler.MSTeams.WebhookURL) > 0:
		eventHandler = new(msteam.MSTeams)
	case len(conf.Handler.Websocket.Url) > 0:
		eventHandler = new(websocket.Websocket)
	case len(conf.Handler.Redis.Url) > 0:
		eventHandler = new(redis.Redis)
	default:
		eventHandler = new(handlers.Default)
	}
	if err := eventHandler.Init(conf); err != nil {
		log.Fatal(err)
	}
	return eventHandler
}

func startWebSocket(conf *config.Config) {
	addr := flag.String("addr", conf.Handler.Websocket.Url, "http service address")
	// ServerStar 启动
	flag.Parse()
	gentleExit()
	// 启动聊天室组件的监听
	go websocket.AliveWS.Run()
	go websocket.AliveWS.AHeartBeat()
	http.HandleFunc("/kubewatch", socketServer)
	log.Printf("监听端口: %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// 监听关闭信号
func gentleExit() {
	// 创建监听退出信号的chan
	c := make(chan os.Signal)
	// 监听
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				ExitFunc()
			//case syscall.SIGUSR1:
			//	fmt.Println("usr1", s)
			//case syscall.SIGUSR2:
			//	fmt.Println("usr2", s)
			default:
				fmt.Println("other", s)
			}
		}
	}()
}

// 退出函数
func ExitFunc() {
	fmt.Println("start close kubewatch server...")
	time.Sleep(200 * time.Millisecond)
	websocket.AliveWS.CloseServer()
	time.Sleep(1 * time.Second)
	fmt.Println("kubewatch server closed!!!")
	os.Exit(0)
}
func socketServer(w http.ResponseWriter, r *http.Request) {
	if !websocket2.IsWebSocketUpgrade(r) {
		log.Println("an illegal connection: %v", r)
		w.Write([]byte(`u is an illegal connection`))
		return
	}

	// 随机生成一个id
	id := randSeq(10)
	client, err := websocket.NewWebSocket(id, w, r)
	checkErr(err)
	defer client.Close()

	welcome2 := fmt.Sprintf("welcome websocket client %s", id)
	client.SendMessage(websocket.ConnectedMessage, welcome2)
	//client.HeartBeat()
	for {
		_, message, err := client.Conn.ReadMessage()
		log.Printf("server receive a message with err: %v %s\n", err, message)
		if websocket2.IsCloseError(err, websocket2.CloseNoStatusReceived, websocket2.CloseAbnormalClosure) {
			log.Println("client ")
			return
		}
		if err != nil {
			log.Println("error:", err)
			return
		}
		// TODO 处理客户端发过来的消息
		//client.Broadcast(string(message))
	}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
