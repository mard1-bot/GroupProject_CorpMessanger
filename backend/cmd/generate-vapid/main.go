package main

import (
	"fmt"
	"log"

	"github.com/SherClockHolmes/webpush-go"
)

func main() {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("VAPID Keys Generated:")
	fmt.Println("Public Key:", publicKey)
	fmt.Println("Private Key:", privateKey)
}
