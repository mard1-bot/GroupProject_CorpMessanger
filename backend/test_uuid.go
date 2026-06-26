package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
)

type Payload struct {
	CalleeID *uuid.UUID `json:"callee_id,omitempty"`
}

func main() {
	js := []byte(`{"callee_id": "c12c7e3a-0502-49c7-8f49-574878a0dfac"}`)
	var p Payload
	err := json.Unmarshal(js, &p)
	if err != nil {
		panic(err)
	}
	if p.CalleeID == nil {
		fmt.Println("CalleeID is nil!")
	} else {
		fmt.Println("CalleeID is", p.CalleeID.String())
	}
}
