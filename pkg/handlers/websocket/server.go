package websocket

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

const (
	// SystemMessage 系统消息
	SystemMessage = iota
	// BroadcastMessage 广播消息(正常的消息)
	BroadcastMessage
	// HeartBeatMessage 心跳消息
	HeartBeatMessage
	// ConnectedMessage 上线通知
	ConnectedMessage
	// DisconnectedMessage 下线通知
	DisconnectedMessage
	// BreakMessage 服务断开链接通知(服务端关闭)
	BreakMessage
)

// AliveList 当前在线列表
type AliveList struct {
	ConnList  map[string]*Client
	register  chan *Client
	destroy   chan *Client
	broadcast chan WebsocketMessage
	cancel    chan int
	Len       int
}

// Client socket客户端
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Type   int // 0表示被动接受所有的事件消息，1表示主动拉取事件消息
	Cancel chan int
}

var (
	upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	AliveWS  *AliveList
	Upgrader = websocket.Upgrader{}
)

func init() {
	// 允许跨域请求
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	AliveWS = NewAliveList()
}

// NewAliveList 初始化
func NewAliveList() *AliveList {
	return &AliveList{
		ConnList:  make(map[string]*Client, 100),
		register:  make(chan *Client, 100),
		destroy:   make(chan *Client, 100),
		broadcast: make(chan WebsocketMessage, 100),
		cancel:    make(chan int),
		Len:       0,
	}
}

// 启动监听
func (al *AliveList) Run() {
	log.Println("开始监听注册事件")
	for {
		select {
		case client := <-al.register:
			log.Println("a client register to server:", client.ID)
			al.ConnList[client.ID] = client
			al.Len++
			al.SysBroadcast(ConnectedMessage, WebsocketMessage{
				ID:      client.ID,
				Content: "connected",
				SendAt:  time.Now().Unix(),
			})

		case client := <-al.destroy:
			log.Println("a client destroy :", client.ID)
			err := client.Conn.Close()
			if err != nil {
				log.Printf("destroy Error: %v \n", err)
			}
			delete(al.ConnList, client.ID)
			al.Len--

		case message := <-al.broadcast:
			for id := range al.ConnList {
				if id != message.ID && al.ConnList[id].Type == 0 {
					log.Printf("brocast message to client : %s %s %d \n", id, message.Content, message.Type)
					err := al.sendMessage(id, message)
					if err != nil {
						log.Println("broadcastError: ", err)
					}
				}
			}
		case sign := <-al.cancel:
			log.Println("cancel event: ", sign)
		}
	}
}

// 关闭, 同时向所有client发送关闭信号
func (al *AliveList) CloseServer() {
	for id, _ := range al.ConnList {
		conn := al.ConnList[id]
		conn.SendMessage(BreakMessage, "")
	}
}

func (al *AliveList) sendMessage(id string, msg WebsocketMessage) error {
	if conn, ok := al.ConnList[id]; ok {
		return conn.SendMessage(msg.Type, msg.Content)
	}
	return fmt.Errorf("conn not found: %v", msg)
}

// Register 注册
func (al *AliveList) Register(client *Client) {
	al.register <- client
}

// Destroy 销毁
func (al *AliveList) Destroy(client *Client) {
	al.destroy <- client
}

// Broadcast 个人广播消息
func (al *AliveList) Broadcast(message WebsocketMessage) {
	al.broadcast <- message
}

// SysBroadcast 系统广播 这里加了一个消息类型, 正常的broadcast应该就是 BroadcastMessage 类型消息
func (al *AliveList) SysBroadcast(messageType int, message WebsocketMessage) {
	message.Type = messageType
	al.Broadcast(message)
}

// Cancel 关闭集合
func (al *AliveList) Cancel() {
	al.cancel <- 1
}

// HeartBeat 服务端发送心跳给客户端(鱼唇)
func (al *AliveList) AHeartBeat() {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			al.Broadcast(WebsocketMessage{
				Content: "heart beat",
				SendAt:  time.Now().Local().Unix(),
				Type:    HeartBeatMessage,
			})
		}
	}
}

// NewWebSocket 新建客户端链接
func NewWebSocket(id string, w http.ResponseWriter, r *http.Request) (client *Client, err error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	cType := 0
	if r.Header.Get("type") == "initiative" {
		cType = 1
	}
	client = &Client{
		Conn:   conn,
		ID:     id,
		Cancel: make(chan int, 1),
		Type:   cType,
	}

	AliveWS.Register(client)
	return
}

// Broadcast 单个客户端的广播事件
func (cli *Client) Broadcast(msg string) {
	AliveWS.Broadcast(WebsocketMessage{
		ID:      cli.ID,
		Content: msg,
		Type:    BroadcastMessage,
		SendAt:  time.Now().Unix(),
	})
}

// SendMessage 单个链接发送消息
func (cli *Client) SendMessage(messageType int, message string) error {
	if messageType == BreakMessage {
		log.Printf("write close msg to client %s", cli.ID)
		err := cli.Conn.WriteMessage(websocket.CloseMessage, []byte("close"))
		return err
	}

	msg := WebsocketMessage{
		ID:      cli.ID,
		Content: message,
		SendAt:  time.Now().Unix(),
		Type:    messageType,
	}
	// 这里固定是
	err := cli.Conn.WriteJSON(msg)
	if err != nil {
		log.Println("sendMessageError :", err)
		log.Println("message: ", msg)
		log.Println("cli: ", cli)
		cli.Close()
	}
	return err
}

// Close 单个链接断开 (这里可以加一个参数, 进行区分关闭链接时的状态, 比如0:正常关闭,1:非正常关闭 etc..)
func (cli *Client) Close() {
	cli.Cancel <- 1
	AliveWS.Broadcast(WebsocketMessage{
		ID:      cli.ID,
		Content: "",
		Type:    DisconnectedMessage,
	})
	AliveWS.Destroy(cli)
}

// HeartBeat 服务端检测链接是否正常 (鱼唇)
func (cli *Client) HeartBeat() {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cli.SendMessage(HeartBeatMessage, "heart beat")
		case <-cli.Cancel:
			log.Println("即将关闭定时器", cli)
			close(cli.Cancel)
			return
		}
	}
}
