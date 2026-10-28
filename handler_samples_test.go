//go:build samples

package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Run with go test -tags samples -run '^TestSampleHexJSON$' -v -count=1.
func TestSampleHexJSON(t *testing.T) {
	file, err := os.Open("samples/hex.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	entries := 0
	for row := 1; scanner.Scan(); row++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entries++
		t.Run(fmt.Sprintf("row_%d", row), func(t *testing.T) {
			entry, err := hex.DecodeString(line)
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}

			client := &sampleMQTTClient{}
			previousMQ := mq
			mq = client
			t.Cleanup(func() { mq = previousMQ })

			// The file contains both raw printer bytes and decoded UTF-8 receipts.
			if !utf8.Valid(entry) || bytes.ContainsAny(entry, "\x00\x1b\x1d") {
				replaySample(t, entry)
			} else {
				t.Fatalf("Invalid input data")
			}

			if len(client.payloads) != 2 {
				t.Fatalf("expected two MQTT payloads (parsed+raw), got %d", len(client.payloads))
			}
			var formatted bytes.Buffer
			if err := json.Indent(&formatted, client.payloads[1], "", "  "); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			t.Logf("generated JSON:\n%s", formatted.String())
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if entries == 0 {
		t.Fatal("samples/hex.txt contains no entries")
	}
}

func replaySample(t *testing.T, entry []byte) {
	t.Helper()

	server, writer := net.Pipe()
	t.Cleanup(func() {
		_ = server.Close()
		_ = writer.Close()
	})
	if err := writer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		handle(server)
	}()
	_, writeErr := writer.Write(entry)
	_ = writer.Close()
	// Closing the writer lets the handler consume the remaining bytes and exit.
	<-done
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

// Embedding the interface keeps this recorder limited to the method handle uses.
type sampleMQTTClient struct {
	mqtt.Client
	payloads [][]byte
}

func (c *sampleMQTTClient) Publish(_ string, _ byte, _ bool, payload interface{}) mqtt.Token {
	c.payloads = append(c.payloads, bytes.Clone(payload.([]byte)))
	return nil
}
