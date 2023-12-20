package main

import (
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

var n = newNode()

func main() {
	n.node.Handle("echo", echo)
	n.node.Handle("broadcast", broadcast)
	n.node.Handle("broadcast_ok", broadcastOk)
	n.node.Handle("read", read)
	n.node.Handle("topology", topology)

	if err := n.node.Run(); err != nil {
		log.Fatal(err)
	}

}

/*
 * Node
 *
 * Holds all the data that the system needs to keep track of.
 */

type Node struct {
	topology      map[string][]string
	uniqueMsgs    []int
	neigbhorMsgs  map[string][]int
	neighborLocks map[string]*uint32
	node          *maelstrom.Node
	lock          sync.Mutex
}

func newNode() Node {
	return Node{
		topology:      make(map[string][]string),
		uniqueMsgs:    make([]int, 0),
		neigbhorMsgs:  make(map[string][]int),
		neighborLocks: make(map[string]*uint32),
		node:          maelstrom.NewNode(),
		lock:          sync.Mutex{},
	}
}

func (n *Node) isMsgNew(msg int) bool {
	contains := false
	for _, m := range n.uniqueMsgs {
		if m == msg {
			contains = true
			break
		}
	}
	return !contains
}

// Add a message
func (n *Node) addMessage(msg int) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.uniqueMsgs = append(n.uniqueMsgs, msg)
}

// Add neighbor message
func (n *Node) addNeighborMsg(name string, msg int) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.neigbhorMsgs[name] = append(n.neigbhorMsgs[name], msg)
}

// Get the messages a neighbor doesn't have
func (n *Node) getNeighborMissingMsgs(name string) []int {
	n.lock.Lock()
	defer n.lock.Unlock()

	missingMsgs := make([]int, 0)
	neighborMsgs := n.neigbhorMsgs[name]

	for _, uniqueMsg := range n.uniqueMsgs {
		missing := true
		for _, msg := range neighborMsgs {
			if uniqueMsg == msg {
				missing = false
				break
			}
		}
		if missing {
			missingMsgs = append(missingMsgs, uniqueMsg)
		}
	}
	return missingMsgs
}

// Make a neighbor lock
func (n *Node) makeNeighborLock(name string) *uint32 {
    n.lock.Lock()
    defer n.lock.Unlock()

    if _, ok := n.neighborLocks[name]; !ok {
        n.neighborLocks[name] = new(uint32)
        return n.neighborLocks[name]
    }
    return n.neighborLocks[name]
}

// Get all the messages
func (n *Node) readMessages() []int {
	n.lock.Lock()
	defer n.lock.Unlock()

	rv := make([]int, len(n.uniqueMsgs))

	for i, val := range n.uniqueMsgs {
		rv[i] = val
	}

	return rv
}

// Store the topology
func (n *Node) storeTopology(topo map[string][]string) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.topology = topo

    for k, _ := range n.neigbhorMsgs {
        n.neigbhorMsgs[k] = make([]int, 0)
        tmp := new(uint32)
        *tmp = 0
        n.neighborLocks[k] = tmp
    }
    log.Println("Neighbor Lock addr: ", n.neighborLocks)
}

// Get all the neighbors of a node
func (n *Node) getNeighbors(name string) []string {
	return n.topology[name]
}

/*
 * RPC Endpoints
 */

func echo(msg maelstrom.Message) error {
	// Unmarshal the message body as an loosely-typed map.
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	// Update the message type to return back.
	body["type"] = "echo_ok"

	// Echo the original message back with the updated message type.
	return n.node.Reply(msg, body)
}

func broadcast(msg maelstrom.Message) error {

	type broadcastMsg struct {
		Type    string `json:"type"`
		Message int    `json:"message"`
	}

	var recv_body broadcastMsg
	if err := json.Unmarshal(msg.Body, &recv_body); err != nil {
		return err
	}

	selfName := msg.Dest

	// Get the message
	uniqueMsg := recv_body.Message

	// Check if the message is new
	if n.isMsgNew(uniqueMsg) {
		// Store it
		n.addMessage(uniqueMsg)

		neighbors := n.getNeighbors(selfName)

		broadcast_body := make(map[string]any)
		broadcast_body["type"] = "broadcast"
		broadcast_body["message"] = uniqueMsg

		for _, neighbor := range neighbors {
			go n.node.Send(neighbor, broadcast_body)
		}
	}

	// all good
	body := make(map[string]any)
	body["type"] = "broadcast_ok"
	if msg.Src[0] == 'n' {
		body["message"] = uniqueMsg
	}

	// broadcast the message to nodes in the topology
	// if the message is new

	return n.node.Reply(msg, body)
}

func broadcastOk(msg maelstrom.Message) error {
	type broadcastOkMsg struct {
		Type      string `json:"type"`
		Message   int    `json:"message"`
		InReplyTo int    `json:"in_reply_to"`
	}

	var recv_body broadcastOkMsg
	if err := json.Unmarshal(msg.Body, &recv_body); err != nil {
		return err
	}

	neighborName := msg.Src

	n.addNeighborMsg(neighborName, recv_body.Message)

	missingMsgs := n.getNeighborMissingMsgs(neighborName)
	if len(missingMsgs) == 0 {
		return nil
	}

    neighborLock, ok := n.neighborLocks[neighborName]
    if !ok {
        neighborLock = n.makeNeighborLock(neighborName)
    }

    log.Println("Neighbor Lock addr: ", neighborLock)

    if !atomic.CompareAndSwapUint32(neighborLock, 0, 1) {
        return nil
    }
    defer atomic.StoreUint32(neighborLock, 0)

	broadcast_body := make(map[string]any)
	broadcast_body["type"] = "broadcast"

	for _, msg := range missingMsgs {
		broadcast_body["message"] = msg
		go n.node.Send(neighborName, broadcast_body)
	}

	return nil
}

func read(msg maelstrom.Message) error {
	// Read all msgs
	uniqueMsgs := n.readMessages()

	// all good
	body := make(map[string]any)
	body["type"] = "read_ok"
	body["messages"] = uniqueMsgs

	return n.node.Reply(msg, body)
}

/*
 * Topology Endpoint:
 *
 * Store the topology of the system
 *
 * Message Format:
 *
 *   {
 *    "type": "topology",
 *    "topology": {
 *      "n1": ["n2", "n3"],
 *      "n2": ["n1"],
 *      "n3": ["n1"]
 *    }
 *  }
 */
func topology(msg maelstrom.Message) error {
	type topologyMsg struct {
		Type     string              `json:"type"`
		Topology map[string][]string `json:"topology"`
	}

	var recv_body topologyMsg
	if err := json.Unmarshal(msg.Body, &recv_body); err != nil {
		return err
	}

	n.storeTopology(recv_body.Topology)

	// all good
	body := make(map[string]any)
	body["type"] = "topology_ok"

	return n.node.Reply(msg, body)
}
