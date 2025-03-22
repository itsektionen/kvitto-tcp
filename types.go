package main

import (
	"encoding/json"
	"time"
)

type Kvitto struct {
	Time      time.Time     `json:"-"`
	Timestamp int64         `json:"timestamp"`
	Sold      []SoldProduct `json:"sold"`
	SoldBy    string        `json:"sold_by"`
}

func (k *Kvitto) publish() {
	k.Timestamp = k.Time.Unix()
	data, _ := json.Marshal(k)
	mq.Publish("kistan/kvitto", 0, false, data)
}

type SoldProduct struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
}
