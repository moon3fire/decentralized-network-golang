package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

var key = []byte("my_own_key") 

type Address struct {
	IPv4 string
	Port string
}

type Node struct {
	Connections map[string]bool
	Address     Address
}

type Package struct {
	To   string
	From string
	Data string
}

// input format should be like ./main :8080
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
	return &Node{
		Connections: make(map[string]bool),
		Address: Address{
			IPv4: splited[0],
			Port: ":" + splited[1],
		},
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
		go handleConnection(node, conn, key)
	}
}

func handleConnection(node *Node, conn net.Conn, key []byte) {
    defer conn.Close()
    var (
        buffer = make([]byte, 512)
        message string
        pack Package
    )
    for {
        length, err := conn.Read(buffer)
        if err != nil {
            break
        }
        message += string(buffer[:length])
    }
    // decrypt the data using Verman cipher
    err := json.Unmarshal([]byte(message), &pack)
    if err != nil {
        return
    }
    decryptedData := make([]byte, len(pack.Data))
    for i := 0; i < len(pack.Data); i++ {
        decryptedData[i] = pack.Data[i] ^ key[i % len(key)]
    }
    pack.Data = string(decryptedData)
    node.ConnectTo([]string{pack.From})
    fmt.Println(pack.Data)
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
		default:
			node.SendMessageToAll(message)
		}
	}
}

func (node *Node) PrintNetwork() {
	for addr := range node.Connections {
		fmt.Println(" | ", addr)
	}
}

func (node *Node) ConnectTo(addresses []string) {
	for _, addr := range addresses {
		node.Connections[addr] = true
	}
}

func (node *Node) SendMessageToAll(message string) {
	var newPack = Package{
		From: node.Address.IPv4 + node.Address.Port,
		Data: message,
	}
	for addr := range node.Connections {
		newPack.To = addr
		node.Send(&newPack, key) 
	}
}

func (node *Node) Send(pack *Package, key []byte) {
    // create a new package
    newPack := Package{
        To: pack.To,
        From: pack.From,
        Data: pack.Data,
    }
    // encrypt the data using Verman cipher
    encryptedData := make([]byte, len(newPack.Data))
    for i := 0; i < len(newPack.Data); i++ {
        encryptedData[i] = newPack.Data[i] ^ key[i % len(key)]
    }
    newPack.Data = string(encryptedData)
    // send the encrypted package
    conn, err := net.Dial("tcp", newPack.To)
    if err != nil {
        delete(node.Connections, newPack.To)
        return
    }
    defer conn.Close()
    jsonPack, _ := json.Marshal(newPack)
    conn.Write(jsonPack)
}


func InputString() string {
	msg, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.Replace(msg, "\n", "", -1)
}

