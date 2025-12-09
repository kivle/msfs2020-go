package websockets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 2048
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func NewUpgrader(allowAllOrigins bool) websocket.Upgrader {
	allowedOrigins := map[string]bool{
		"https://kivle.github.io":           true,
		"https://kivle.github.io/msfs-map":  true,
		"https://github.com/kivle/msfs-map": true,
	}
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if allowAllOrigins {
				return true
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return false
			}
			originLower := strings.ToLower(origin)
			if isAllowedLocalOrigin(originLower) {
				return true
			}
			if allowedOrigins[originLower] {
				return true
			}
			return false
		},
	}
}

func isAllowedLocalOrigin(origin string) bool {
	// Accept http(s)://localhost or loopback IPs
	if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "https://localhost") {
		return true
	}
	if strings.HasPrefix(origin, "http://127.0.0.1") || strings.HasPrefix(origin, "https://127.0.0.1") {
		return true
	}
	// basic check for other loopback IPs (IPv4/IPv6)
	if strings.HasPrefix(origin, "http://[::1]") || strings.HasPrefix(origin, "https://[::1]") {
		return true
	}
	// Allow http/https origins with RFC1918 IPs
	if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
		host := strings.TrimPrefix(origin, "http://")
		host = strings.TrimPrefix(host, "https://")
		hostPort := strings.SplitN(host, "/", 2)[0]
		hostOnly := strings.Split(hostPort, ":")[0]
		if ip := net.ParseIP(hostOnly); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() {
				return true
			}
		}
	}
	return false
}

type ReceiveMessage struct {
	Message    []byte
	Connection *Connection
}

type Connection struct {
	socket    *Websocket
	conn      *websocket.Conn
	Send      chan []byte
	SendQueue chan []byte
}

func (c *Connection) Run() {
	go c.readPump()
	go c.writer()
	c.writePump()
}

func (c *Connection) SendPacket(data map[string]interface{}) {
	buf, _ := json.Marshal(data)
	c.Send <- buf
}

func (c *Connection) SendError(target string, msg string) {
	pkt := map[string]string{"target": target, "type": "error", "message": msg}
	buf, _ := json.Marshal(pkt)
	c.Send <- buf
}

func (c *Connection) readPump() {
	defer func() {
		c.socket.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				//log.Printf("error: %v", err)
			}
			//log.Printf("error: %v", err)
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.socket.ReceiveMessages <- ReceiveMessage{
			Message:    message,
			Connection: c,
		}
	}
}

func (c *Connection) writer() {
	var buf bytes.Buffer
	flushticker := time.NewTicker(time.Millisecond * 16)

	defer func() {
		flushticker.Stop()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				close(c.SendQueue)
				return
			}
			buf.Write(message)
			buf.WriteString("\n")

		case <-flushticker.C:
			message, err := buf.ReadBytes('\n')
			if err == nil {
				c.SendQueue <- message
			}
		}
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.SendQueue:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				fmt.Println(err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}
