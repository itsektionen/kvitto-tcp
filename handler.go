package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strings"
)

const (
	groupNumber   = "number"
	groupCategory = "category"
	groupCount    = "count"
	groupProduct  = "product"
	groupExtra    = "extra"
	groupUser     = "user"
	groupDate     = "date"
)

var (
	// re         = regexp.MustCompile(`([A-Za-z]*)[:Nr. ]*? (\d*)\n*(\d+-\d+-\d+ \d+:\d+)\n+[-]{22}|-\n[ ]*([a-zA-ZÅÄÖåäö]*)\n*|([0-9-]*) st\n+([0-9A-Za-z ÅÄÖåäö&#-().!*é]*)\n|[A-Za-zåäöÅÄÖ]+ av:\n([a-z-A-Z]*)`)
	// re         = regexp.MustCompile(`\n*(\d+-\d+-\d+ \d+:\d+)\n+-{22}|-\n *(?P<category>[a-zA-ZÅÄÖåäö]*)\n*|(?P<count>[0-9-]*) st\n+(?P<product>[0-9A-Za-z ÅÄÖåäö&#-().!*é]*)\n|[A-Za-zåäöÅÄÖ]+ av: (?P<user>[a-z-A-Z]*)|Beställd: (?P<date>[0-9a-z :]+)`)
	re            = regexp.MustCompile(`\n*(\d+-\d+-\d+ \d+:\d+)\n+-{22}|Nr. *(?P<number>\d+)\n|-\n *(?P<category>[a-zA-ZÅÄÖåäö]*)\n*|(?P<count>[0-9-]*) st\n+(?P<product>[0-9A-Za-z ÅÄÖåäö&#-().!*é]*)\n(?P<extra>[A-Za-zåäö\n ]+)\n|[A-Za-zåäöÅÄÖ]+ av: (?P<user>[a-z-A-Z]*)|Beställd: (?P<date>[0-9a-z :]+)`)
	re2           = regexp.MustCompile("[0-9]{2}:[0-9]{2}") // Regexp for clock at end of receipt
	monthReplacer = strings.NewReplacer(
		" jan ", " Jan ",
		" feb ", " Feb ",
		" mar ", " Mar ",
		" apr ", " Apr ",
		" maj ", " May ",
		" jun ", " Jun ",
		" jul ", " Jul ",
		" aug ", " Aug ",
		" sep ", " Sep ",
		" okt ", " Oct ",
		" nov ", " Nov ",
		" dec ", " Dec ",
	)
)

func handle(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()
	log.Println("Client connected:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	combinedBuffer := bytes.Buffer{}

	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			log.Println("Client disconnected:", conn.RemoteAddr())
			break
		}
		if err != nil {
			println("Failed to read from buffer:", err.Error())
			return
		}
		err = combinedBuffer.WriteByte(b)
		if err != nil {
			println("Failed to write to combined buffer:", err.Error())
			return
		}
	}
	message := combinedBuffer.Bytes()
	mq.Publish("kistan/kvitto/raw", 0, false, message)

	res, err := createKvitto(message)

	if err != nil {
		log.Println("Could not create data out:", err.Error(), "raw:\n", formatHex(message))
		return
	}
	res.publish()
}

func formatHex(data []byte) string {
	out := ""
	for _, b := range data {
		out += fmt.Sprintf("%02x", b)
	}
	return out
}
