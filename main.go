package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
)

var key = []byte("x52dmid220NYDo2kd29")

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
	To   string
	From string
	Data string
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
			node.pingNode(splited[1:])
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
		if !node.Connections[addr] {
			node.Connections[addr] = true
			fmt.Printf("Connected to %s\n", addr)
		} else {
			fmt.Printf("Already connected to %s\n", addr)
		}
	}
}

func (node *Node) SendMessageToAll(message string) {
	var newPack = Package{
		From: node.Address.IPv4 + node.Address.Port,
		Data: message,
	}
	for addr := range node.Connections {
		newPack.To = addr
		node.Send(&newPack)
	}
}

func (node *Node) Send(pack *Package) {
	newPack := Package{
		To:   pack.To,
		From: pack.From,
		Data: pack.Data,
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

func InputString() string {
	msg, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.Replace(msg, "\n", "", -1)
}

func (node *Node) pingNode(addresses []string) {
	for _, addr := range addresses {
		if node.Address.IPv4+node.Address.Port == addr {
			fmt.Println("You cannot ping yourself.")
			continue
		}
		node.pingNodeSingle(addr)
	}
}

func (node *Node) pingNodeSingle(address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("You can connect to %s\n", address)
		return
	}
	defer conn.Close()

	pack := Package{
		To:   address,
		From: node.Address.IPv4 + node.Address.Port,
	}

	jsonPack, _ := json.Marshal(pack)
	conn.Write(jsonPack)
}

func NewRoutingTable() *RoutingTable {
	routingTable := &RoutingTable{}

	for i := 0; i < KeySize*8; i++ {
		routingTable.Buckets[i] = make([]*Node, 0)
	}

	return routingTable
}

func compareNodeDistance(target, a, b NodeID) bool {
	distanceA := nodeDistance(target, a)
	distanceB := nodeDistance(target, b)

	for i := 0; i < KeySize; i++ {
		if distanceA[i] != distanceB[i] {
			return distanceA[i] < distanceB[i]
		}
	}
	return false
}

func (table *RoutingTable) FindClosestNodes(target NodeID) []*Node {
	var closestNodes []*Node
	bucketIndex := table.getBucketIndex(target)

	closestNodes = append(closestNodes, table.Buckets[bucketIndex]...)

	sort.SliceStable(closestNodes, func(i, j int) bool {
		return compareNodeDistance(target, closestNodes[i].ID, closestNodes[j].ID)
	})

	remainingNodes := K - len(closestNodes)
	if remainingNodes > 0 {
		for i := bucketIndex + 1; i < len(table.Buckets) && remainingNodes > 0; i++ {
			closestNodes = append(closestNodes, table.Buckets[i]...)
			remainingNodes -= len(table.Buckets[i])
			if remainingNodes < 0 {
				sort.SliceStable(closestNodes, func(i, j int) bool {
					return compareNodeDistance(target, closestNodes[i].ID, closestNodes[j].ID)
				})
				closestNodes = closestNodes[:K]
			}
		}
	}

	return closestNodes
}

func (table *RoutingTable) AddNode(node *Node) {
	bucketIndex := table.getBucketIndex(node.ID)
	bucket := table.Buckets[bucketIndex]

	for _, n := range bucket {
		if n.ID == node.ID {
			return
		}
	}

	if len(bucket) < BucketSize {
		table.Buckets[bucketIndex] = append(bucket, node)
		return
	}

	replaceIndex := table.getReplacementIndex(bucket, node.ID)
	table.Buckets[bucketIndex][replaceIndex] = node
}

func (table *RoutingTable) getBucketIndex(nodeID NodeID) int {
	for i := 0; i < KeySize*8; i++ {
		if nodeID[i/8]&(1<<(7-(i%8))) != table.Buckets[i][0].ID[i/8]&(1<<(7-(i%8))) {
			return i
		}
	}
	return KeySize*8 - 1
}

func (table *RoutingTable) getReplacementIndex(bucket []*Node, nodeID NodeID) int {
	target := nodeID
	for i, node := range bucket {
		if compareNodeDistance(target, node.ID, bucket[i].ID) {
			return i
		}
	}
	return 0
}

func nodeDistance(a, b NodeID) NodeID {
	var result NodeID
	for i := 0; i < KeySize; i++ {
		result[i] = a[i] ^ b[i]
	}
	return result
}

