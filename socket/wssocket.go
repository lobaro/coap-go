package sckt

import (
	"github.com/gorilla/websocket"
	"net/http"
	"log"
	"strconv"
	"net"
	"fmt"
	"errors"
)

type wsSocket struct {
	id        int
	
	Port      int
	Uri       string
	onRx      chan <- *Datagram
	wscon     *websocket.Conn
	wsCons    map[string]*websocket.Conn
	localaddr net.Addr
}

func NewWSSocket(SocketID int, Uri string, Port int, chRx chan <-*Datagram) (Socket, error) {
	sckt := new(wsSocket)
	sckt.Port = Port
	sckt.Uri = Uri
	sckt.onRx = chRx
	sckt.id = SocketID
	sckt.wsCons = make(map[string]*websocket.Conn)

	return sckt, nil
}

func (sckt *wsSocket) Write(data []byte, Addr net.Addr) (int, error) {

	conn := sckt.wsCons[Addr.String()]

	if (conn == nil) {
		return 0, errors.New("No open WS-Connection for this Remote Address!")
	}

	fmt.Printf("Writing to ws-Socket on %s\r\n", Addr.String())

	err := conn.WriteMessage(websocket.BinaryMessage, data)
	n := len(data)

	if (err != nil) {
		n = 0
		fmt.Println(err)
	}

	return n, err
}

func (sckt *wsSocket)Close() error {
	return sckt.Close()
}

func (sckt *wsSocket)SocketID() int {
	return sckt.id
}

func (sckt *wsSocket)ReceiveCh(newChan chan <-*Datagram) chan <-*Datagram {
	if newChan != nil {
		sckt.onRx = newChan
	}
	return sckt.onRx
}

func (sckt *wsSocket) reqHandler(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}
	sckt.wsCons[conn.RemoteAddr().String()] = conn

	defer conn.Close()
	defer delete(sckt.wsCons, conn.RemoteAddr().String())

	fmt.Printf("New WS-Connection to %s from %s\r\n", conn.LocalAddr().String(), conn.RemoteAddr().String())
	fmt.Println("My Connections now:\r\n", sckt.wsCons)

	for {
		//messageType, binData, err := conn.ReadMessage()
		messageType, binData, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			fmt.Println("Connection Error or Close. My Connections now:\r\n", sckt.wsCons)
			return
		}

		if (messageType != websocket.BinaryMessage || sckt.onRx == nil) {
			fmt.Printf("continue")
			continue
		}

		fmt.Printf("got data! type:%d\r\n", messageType)

		bCpy := make([]byte, len(binData))
		copy(bCpy, binData)

		packet := Datagram{}
		packet.Socket = sckt
		packet.Data = bCpy
		//packet.Destination = conn.LocalAddr()
		packet.Origin = conn.RemoteAddr()

		sckt.onRx <- &packet //send away the packet	
	}
}

func (sckt *wsSocket) AsyncListenAndServe() {

	fmt.Println("started websocket coap mirror server \"" + sckt.Uri + "\" on Port " + strconv.Itoa(sckt.Port))
	http.HandleFunc(sckt.Uri, sckt.reqHandler)
	http.Handle("/", http.FileServer(http.Dir(".")))
	go http.ListenAndServe(":" + strconv.Itoa(sckt.Port), nil)
}

func (sckt *wsSocket) Network() string {
	return "ws over TCP"
}

func (sckt *wsSocket)String() string {
	return sckt.Uri + ":" + strconv.Itoa(sckt.Port)
}

func (sckt *wsSocket) LocalAddr() net.Addr {
	return sckt
}

