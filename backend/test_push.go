package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/SherClockHolmes/webpush-go"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := "postgres://corpmessenger:corpmessenger_password@127.0.0.1:5433/corpmessenger?sslmode=disable"
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT endpoint, p256dh, auth FROM web_push_subscriptions")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	pubKey := "BDxQrS09IYxG9Za7P2VJ4mRBi1yASey-8PqskldZypnoL_Rqu6EMMy1n7UWxXAjrpyQxzMuGtoj_7MONOnQUVXw"
	privKey := "4nnPtrRxpy5kX0iMXgSqnY-YbxOM0N8laoWrv3dpPtw"

	count := 0
	for rows.Next() {
		var endpoint, p256, auth string
		if err := rows.Scan(&endpoint, &p256, &auth); err != nil {
			log.Fatal(err)
		}

		sub := &webpush.Subscription{
			Endpoint: endpoint,
			Keys: webpush.Keys{
				P256dh: p256,
				Auth:   auth,
			},
		}

		resp, err := webpush.SendNotification([]byte(`{"title":"Test","body":"It works!"}`), sub, &webpush.Options{
			Subscriber:      "mailto:admin@corpmessenger.com",
			VAPIDPublicKey:  pubKey,
			VAPIDPrivateKey: privKey,
			TTL:             30,
		})
		if err != nil {
			fmt.Printf("Error sending to %s: %v\n", endpoint, err)
		} else {
			fmt.Printf("Success sending to %s (Status: %d)\n", endpoint, resp.StatusCode)
		}
		count++
	}
	fmt.Printf("Total subscriptions: %d\n", count)
}
