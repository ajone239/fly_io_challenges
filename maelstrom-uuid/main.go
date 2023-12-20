package main

import (
  "fmt"
  "log"
  "time"
  "crypto/sha256"

  maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

var n = maelstrom.NewNode()

func main() {

  n.Handle("generate", generate)

  if err := n.Run(); err != nil {
    log.Fatal(err)
  }

}

func generate (msg maelstrom.Message) error {
    src :=  msg.Src
    dest := msg.Dest

    // Hash src and dest with current time to generate a unique id.
    input_string := src + dest + time.Now().String()
    hash := sha256.Sum256([]byte(input_string))
    hash_string := fmt.Sprintf("%x", hash)

    log.Printf("Generated id %s for message from %s to %s", hash_string, src, dest)

    body := make(map[string]any)
    body["type"] = "generate_ok"
    body["id"] = hash_string

    // Echo the original message back with the updated message type.
    return n.Reply(msg, body)
}
