package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ybbus/jsonrpc/v2"
)

var addr = flag.String("addr", "localhost:8910", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/"}
	log.Printf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// err = sendListProductRq(conn)

	// if err != nil {
	// 	log.Printf("Err %+vn", err)
	// 	return
	// }

	// TODO: should be check get data from FIX
	err = sendUpdatePriceRq(conn, "33ugpDWbC2mLrYSQvu1BHfykR8bt3MVc4S3YuuXMVRH3", 169920000, 730000, "trading")
	if err != nil {
		log.Printf("Err %+vn", err)
		return
	}

	select {
	case <-done:
		return
	case <-interrupt:
		log.Println("interrupt")

		// Cleanly close the connection by sending a close message and then
		// waiting (with timeout) for the server to close the connection.
		err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("write close:", err)
			return
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		return
	}
}

func sendUpdatePriceRq(conn *websocket.Conn, account string, price uint64, conf uint32, status string) error {
	params := make(map[string]interface{}, 0)
	params["account"] = account
	params["price"] = price
	params["conf"] = conf
	params["status"] = status

	updatePriceRq := jsonrpc.NewRequest("update_price", params)
	b, err := json.Marshal(updatePriceRq)
	if err != nil {
		return fmt.Errorf("Err marsharl updatePriceRq %+v", err)
	}
	err = conn.WriteMessage(websocket.TextMessage, b)
	if err != nil {
		return fmt.Errorf("Err Write  update_price %+v", err)
	}
	return nil
}

func sendSubscribePriceRq(conn *websocket.Conn, account string) error {
	params := make(map[string]interface{}, 0)
	params["account"] = account

	updatePriceRq := jsonrpc.NewRequest("subscribe_price", params)
	b, err := json.Marshal(updatePriceRq)
	if err != nil {
		return fmt.Errorf("Err marsharl updatePriceRq %+v", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, b)
	if err != nil {
		return fmt.Errorf("Err Write subscribe_price %+v", err)
	}
	return nil
}

func sendListProductRq(conn *websocket.Conn) error {
	productListRq := jsonrpc.NewRequest("get_product_list", nil)
	b, err := json.Marshal(productListRq)
	if err != nil {
		return fmt.Errorf("Err marsharl getProductList %+v", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, b)
	if err != nil {
		return fmt.Errorf("Err Write get_product_list %+v", err)
	}
	return nil
}
