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
	var p Payload
	err := json.Unmarshal([]byte(`{"callee_id": ""}`), &p)
	fmt.Printf("Error: %v\n", err)
}
