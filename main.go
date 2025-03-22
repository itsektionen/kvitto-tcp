package main

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"net"
	"time"
)

var mq mqtt.Client

func main() {
	if err := setupMQTT(); err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("tcp4", "0.0.0.0:4321")
	if err != nil {
		log.Fatal("Could not open listener:", err)
	}
	defer func() {
		_ = l.Close()
	}()

	log.Println("Listening:", l.Addr())

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Could not accept connection:", err)
			continue

		}

		go handle(conn)
	}
}

func setupMQTT() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://server.insektionen.se:1883")
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)
	opts.SetPingTimeout(time.Second * 30)

	mq = mqtt.NewClient(opts)

	for t := mq.Connect(); t.Error() != nil; t = mq.Connect() {
		log.Println("MQTT: connecting...")
		time.Sleep(time.Second * 5)
	}

	return nil
}
