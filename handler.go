package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// re       = regexp.MustCompile(`([A-Za-z]*)[:Nr. ]*? (\d*)\n*(\d+-\d+-\d+ \d+:\d+)\n+[-]{22}|-\n[ ]*([a-zA-ZÅÄÖåäö]*)\n*|([0-9-]*) st\n+([0-9A-Za-z ÅÄÖåäö&#-().!*é]*)\n|[A-Za-zåäöÅÄÖ]+ av:\n([a-z-A-Z]*)`)
	re  = regexp.MustCompile(`\n*(\d+-\d+-\d+ \d+:\d+)\n+-{22}|-\n *(?P<category>[a-zA-ZÅÄÖåäö]*)\n*|(?P<count>[0-9-]*) st\n+(?P<product>[0-9A-Za-z ÅÄÖåäö&#-().!*é]*)\n|[A-Za-zåäöÅÄÖ]+ av: (?P<user>[a-z-A-Z]*)|Beställd: (?P<date>[0-9a-z :]+)`)
	re2 = regexp.MustCompile("[0-9]{2}:[0-9]{2}") // Regexp for clock at end of receipt
)

func handle(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()
	log.Println("Client connected:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	for {
		var (
			buffer bytes.Buffer
			b      byte
			err    error
		)
		for b, err = reader.ReadByte(); err == nil; b, err = reader.ReadByte() {
			switch b {
			case 0x1b:
				_, _ = reader.Discard(2)
			case 0x1d:
				_, _ = reader.Discard(2)
			case 0xe5: // å
				buffer.Write([]byte{0xc3, 0xa5})
			case 0xe4: // ä
				buffer.Write([]byte{0xc3, 0xa4})
			case 0xf6: // ö
				buffer.Write([]byte{0xc3, 0xb6})

			case 0xc5: // Å
				buffer.Write([]byte{0xc3, 0x85})
			case 0xc4: // Ä
				buffer.Write([]byte{0xc3, 0x84})
			case 0xd6: // Ö
				buffer.Write([]byte{0xc3, 0x96})

			case 0x0D, 0x00:
				// Skip
			default:
				buffer.WriteByte(b)
			}

			if re2.FindString(buffer.String()) != "" {
				break
			}
		}
		if err == io.EOF {
			break
		}

		res, err := createKvitto(buffer.String())
		if err != nil {
			log.Println("Could not create data out:", err)
			continue
		}
		res.publish()
	}

	log.Println("Client disconnected:", conn.RemoteAddr())
}

func createKvitto(text string) (*Kvitto, error) {
	text = strings.ReplaceAll(text, "\r", "")
	reRes := re.FindAllStringSubmatch(text, -1)
	ent := &Kvitto{
		Time:   time.Now(),
		Sold:   make([]SoldProduct, 0, 5),
		SoldBy: "",
	}

	loc, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		return nil, fmt.Errorf("unable to load location: %v", err)
	}

	category := ""
	for _, v := range reRes {
		if strings.Contains(v[0], "Beställd av") {
			ent.SoldBy = v[5]
		} else if strings.Contains(v[0], "Beställd") {
			t, err := time.ParseInLocation("02 Jan 15:04", v[6], loc)
			if err != nil {
				return nil, fmt.Errorf("unable to parse time: %v", err)
			}
			t = t.AddDate(time.Now().Year(), 0, 0)
			ent.Time = t
		} else if strings.HasPrefix(v[0], "-") {
			if v[2] != "" {
				category = v[2]
			}
		} else {
			count, _ := strconv.Atoi(v[3])
			prod := SoldProduct{
				Category: category,
				Name:     v[4],
				Count:    count,
			}
			ent.Sold = append(ent.Sold, prod)
		}
	}

	return ent, nil
}
