package websocket

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"kubewatch/config"
	kbEvent "kubewatch/pkg/event"
	"time"
)

var WebsocketErrMsg = `
%s

You need to set Websocket url
using "--port/-p" or using environment variables:

export KW_WEBSOCKET_PORT=5000

Command line flags will override environment variables

`
// WebsocketMessage for messages
type WebsocketMessage struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	SendAt  int64  `json:"sendat"`
	Type    int    `json:"type"`
}
// Websocket handler implements handler.Handler interface,
// Notify event to Websocket channel
type Websocket struct {
	Url string
}

// Init prepares Websocket configuration
func (m *Websocket) Init(c *config.Config) error {
	return checkMissingWebsocketVars(m)
}

// ObjectCreated calls notifyWebsocket on event creation
func (m *Websocket) ObjectCreated(obj interface{}) {
	notifyWebsocket(obj, "created")
}

// ObjectDeleted calls notifyWebsocket on event creation
func (m *Websocket) ObjectDeleted(obj interface{}) {
	notifyWebsocket(obj, "deleted")
}

// ObjectUpdated calls notifyWebsocket on event creation
func (m *Websocket) ObjectUpdated(oldObj, newObj interface{}) {
	notifyWebsocket(newObj, "updated")
}

// TestHandler tests the handler configurarion by sending test messages.
func (m *Websocket) TestHandler() {
}

func notifyWebsocket(obj interface{}, action string) {
	e := kbEvent.New(obj, action)
	msg, err := json.Marshal(e)
	if err != nil {
		logrus.Errorf("json marshal event error %v", err)
	}
	AliveWS.Broadcast(WebsocketMessage{
		Content: string(msg),
		SendAt:  time.Now().Unix(),
		Type:    BroadcastMessage,
	})
}

func checkMissingWebsocketVars(s *Websocket) error {
	return nil
}
