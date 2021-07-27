package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/interviewparrot/OpenAVStream/pkg/mediaserver"
	"github.com/interviewparrot/OpenAVStream/pkg/mediastream"
	"github.com/zserge/lorca"
)

//go:embed three.js
var fs embed.FS

// Go types that are bound to the UI must be thread-safe, because each binding
// is executed in its own goroutine. In this simple case we may use atomic
// operations, but for more complex cases one should use proper synchronization.
type counter struct {
	sync.Mutex
	count int
}

func (c *counter) Add(n int) {
	c.Lock()
	defer c.Unlock()
	c.count = c.count + n
}

func (c *counter) Value() int {
	c.Lock()
	defer c.Unlock()
	return c.count
}

// Lorca sends data with this type, tests indicate
type idxData map[string]interface{}
type vrxData map[string]interface{}

func IdxBff(data idxData) {
	fmt.Println(time.Now())
	fmt.Println(`Index buffer data length: `, len(data))
}

func VrxBff(data vrxData) {
	fmt.Println(time.Now())
	fmt.Println(`Vertex buffer data length: `, len(data))
}

func main() {
	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "--class=Lorca")
	}
	ui, err := lorca.New("", "", 480, 320, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	// A simple way to know when UI is ready (uses body.onload event in JS)
	ui.Bind("start", func() {
		log.Println("UI is ready")
	})

	// Create and bind Go object to the UI
	c := &counter{}
	ui.Bind("counterAdd", c.Add)
	ui.Bind("counterValue", c.Value)

	// To pass array from JS to Go
	ui.Bind("vrxBff", VrxBff)
	ui.Bind("idxBff", IdxBff)

	// Load HTML.
	// You may also use `data:text/html,<base64>` approach to load initial HTML,
	// e.g: ui.Load("data:text/html," + url.PathEscape(html))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.FileServer(http.FS(fs)))
	ui.Load(fmt.Sprintf("http://%s/three.js/editor", ln.Addr()))

	// You may use console.log to debug your JS code, it will be printed via
	// log.Println(). Also exceptions are printed in a similar manner.
	ui.Eval(`
		console.log("Hello, world!");
		console.log('Multiple values:', [1, false, {"x":5}]);
	`)

	// Launch WebSocket server before the next "wait" logic
	// Test WebSocket streaming
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/session", sessionHandler)
	log.Fatal(http.ListenAndServe(":4040", nil))

	// Wait until the interrupt signal arrives or browser window is closed
	sigc := make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt)
	select {
	case <-sigc:
	case <-ui.Done():
	}

	log.Println("exiting...")
}

var upgrader = websocket.Upgrader{} // use default options

func ProcessMessage(msg []byte) {
	log.Println("handle incoming bytes")
	clientMessage := mediaserver.ClientMsg{}
	json.Unmarshal(msg, &clientMessage)
	if mediaserver.IsSessionExist(clientMessage.SessionId) {
		session := mediaserver.SessionStore[clientMessage.SessionId]
		switch cmd := clientMessage.Command; cmd {
		case mediaserver.CMD_ReceiveChunk:
			data, err := base64.StdEncoding.DecodeString(clientMessage.Data)
			log.Println("receiving chunk for sessionID: " + clientMessage.SessionId + " and session state is: " + session.State)
			if err != nil {
				fmt.Println("error:", err)
				return
			}
			mediastream.ProcessIncomingMsg(session, data)
		}
	}
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	c, err := upgrader.Upgrade(w, r, nil)
	session := mediaserver.CreateNewSession(c)
	// Send the session id to the client
	msg := mediaserver.ServerMsg{mediaserver.CMD_ReceiveSessionId, session.SessionId, session.SessionId}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		fmt.Println(err)
	}
	c.WriteMessage(1, msgBytes)

	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {

			log.Println("read:", err)
			break
		}
		log.Printf("message type: %s", mt)
		if mt == 2 {
			log.Println("Cannot process binary message right now")
		} else {
			ProcessMessage(message)
		}
	}
}
