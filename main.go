package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

var key = []byte("C0z1IFc8BtDNeaOqSnVmMg9l3JxH5XZdR6vWp7b4YrLyGwuiKUfTsAj2kPQhE8BzrLMf2XaR")

const (
	KeySize         = 16
	BucketSize      = 20
	ReplicationSize = 10
	K               = 10
)

type NodeID [KeySize]byte

type RoutingTable struct {
	Buckets [KeySize * 8][]*Node
}

type Address struct {
	IPv4 string
	Port string
}

type Node struct {
	ID           NodeID
	Address      Address
	Connections  map[string]bool
	RoutingTable *RoutingTable
	DataStore    map[string]string
}

type Package struct {
	To        string
	From      string
	Data      string
	Broadcast bool
}

func init() {
	if len(os.Args) != 2 {
		panic("len args != 2")
	}
}

func main() {
	NewNode(os.Args[1]).Run(handleServer, handleClient)
}

func NewNode(address string) *Node {
	splited := strings.Split(address, ":")
	if len(splited) != 2 {
		return nil
	}
	var id NodeID
	copy(id[:], []byte(splited[0]))
	return &Node{
		ID:           id,
		Address:      Address{IPv4: splited[0], Port: ":" + splited[1]},
		Connections:  make(map[string]bool),
		RoutingTable: NewRoutingTable(),
		DataStore:    make(map[string]string),
	}
}

func (node *Node) Run(handleServer func(*Node), handleClient func(*Node)) {
	go handleServer(node)
	handleClient(node)
}

func handleServer(node *Node) {
	listen, err := net.Listen("tcp", "0.0.0.0"+node.Address.Port)
	if err != nil {
		panic("listen error")
	}
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			break
		}
		go handleConnection(node, conn)
	}
}

func handleConnection(node *Node, conn net.Conn) {
	defer conn.Close()
	var (
		buffer  = make([]byte, 512)
		message string
		pack    Package
	)
	for {
		length, err := conn.Read(buffer)
		if err != nil {
			break
		}
		message += string(buffer[:length])
	}
	err := json.Unmarshal([]byte(message), &pack)
	if err != nil {
		return
	}
	decryptedData := make([]byte, len(pack.Data))
	for i := 0; i < len(pack.Data); i++ {
		decryptedData[i] = pack.Data[i] ^ key[i%len(key)]
	}
	pack.Data = string(decryptedData)
	node.ConnectTo([]string{pack.From})
	fmt.Println(pack.Data)
	if pack.Broadcast {
		node.SendMessageToAll(pack.Data)
	}
}

func handleClient(node *Node) {
	for {
		message := InputString()
		splited := strings.Split(message, " ")
		switch splited[0] {
		case "/exit":
			os.Exit(0)
		case "/connect":
			node.ConnectTo(splited[1:])
		case "/network":
			node.PrintNetwork()
		case "/ping":
			node.Ping(splited[1])
		default:
			isBroadcast := AskForBroadcast()
			if isBroadcast {
				node.SendMessageToAll(message)
			} else {
				node.SendMessage(message, false) // changed to false
			}
		}
	}
}

func (node *Node) FindClosestNodes(target NodeID, count int) []*Node {
	var result []*Node
	bucketIndex := node.GetBucketIndex(target)
	result = append(result, node.RoutingTable.Buckets[bucketIndex]...)
	for i := bucketIndex - 1; i >= 0; i-- {
		result = append(result, node.RoutingTable.Buckets[i]...)
		if len(result) >= count {
			break
		}
	}
	for i := bucketIndex + 1; i < len(node.RoutingTable.Buckets); i++ {
		result = append(result, node.RoutingTable.Buckets[i]...)
		if len(result) >= count {
			break
		}
	}
	return result[:min(count, len(result))]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (node *Node) GetBucketIndex(target NodeID) int {
	for i := 0; i < KeySize*8; i++ {
		if target[i/8]&(1<<(7-uint(i%8))) != node.ID[i/8]&(1<<(7-uint(i%8))) {
			return i
		}
	}
	return KeySize * 8
}

func (node *Node) ConnectTo(addresses []string) {
	for _, addr := range addresses {
		node.Connections[addr] = true
	}
}

func (node *Node) PrintNetwork() {
	for addr := range node.Connections {
		fmt.Printf("Connected to: %s\n", addr)
	}
}

func NewRoutingTable() *RoutingTable {
	var routingTable RoutingTable
	for i := range routingTable.Buckets {
		routingTable.Buckets[i] = make([]*Node, 0)
	}
	return &routingTable
}

func (node *Node) SendMessageToAll(message string) {
	node.SendMessage(message, true)
}

func (node *Node) SendMessage(message string, broadcast bool) {
	var newPack = Package{
		From:      node.Address.IPv4 + node.Address.Port,
		Data:      message,
		Broadcast: broadcast,
	}
	for addr := range node.Connections {
		newPack.To = addr
		node.Send(&newPack)
	}
}

func (node *Node) Send(pack *Package) {
	newPack := Package{
		To:        pack.To,
		From:      pack.From,
		Data:      pack.Data,
		Broadcast: pack.Broadcast,
	}
	encryptedData := make([]byte, len(newPack.Data))
	for i := 0; i < len(newPack.Data); i++ {
		encryptedData[i] = newPack.Data[i] ^ key[i%len(key)]
	}
	newPack.Data = string(encryptedData)
	conn, err := net.Dial("tcp", newPack.To)
	if err != nil {
		delete(node.Connections, newPack.To)
		return
	}
	defer conn.Close()
	jsonPack, _ := json.Marshal(newPack)
	conn.Write(jsonPack)
}

func (node *Node) Ping(address string) {
	_, ok := node.Connections[address]
	if ok {
		fmt.Printf("You are already connected to %s\n", address)
		return
	}
	node.Send(&Package{
		To:        address,
		From:      node.Address.IPv4 + node.Address.Port,
		Data:      "PING " + node.Address.Port,
		Broadcast: false,
	})
	fmt.Println("Node " + address + " is available to connect with");
}

func InputString() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func AskForBroadcast() bool {
	fmt.Println("Is the message being broadcasted? (yes/no)")
	answer := InputString()
	return strings.ToLower(answer) == "yes"
}

