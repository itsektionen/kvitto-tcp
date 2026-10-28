package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BongType int

const (
	BongUnknown BongType = 0
	BongKvitto           = 1
	BongMessage          = 2
)

type BongParser struct {
	tokenizer         Tokenizer
	bongType          BongType
	kvitto            Kvitto
	message           Message
	currentCategory   string
	currentTokenIndex int
}

func parseMessage(message []byte) {
	currentFormat := TextFormat{
		bold:          false,
		characterSize: 0,
		underline:     UnderlineNone,
		alignment:     AlignLeft,
	}

	kvittoParser := BongParser{
		tokenizer: Tokenizer{
			tokenBlocks: make(TokenBlocks, 0),
			currentToken: TokenBlock{
				format:      currentFormat,
				text:        "",
				specialType: Text,
			},
			reader: bytes.NewReader(message),
		},
		currentCategory:   "NOCATEGORY",
		currentTokenIndex: 0,
		kvitto: Kvitto{
			Sold: make([]SoldProduct, 0),
		},
	}
	if err := kvittoParser.Parse(); err != nil {
		kvittoParser.tokenizer.print()

		log.Println("Could not create data out:", err.Error(), "raw:\n", formatHex(message))
	}
	switch kvittoParser.bongType {
	case BongKvitto:
		kvittoParser.kvitto.publish()
	case BongMessage:
		kvittoParser.message.publish()
	default:
		log.Println("Invalid bong type:", kvittoParser.bongType)
	}

}

func (p *BongParser) Parse() error {
	if err := p.tokenizer.readToEnd(); err != nil {
		return fmt.Errorf("Tokenizer error %w", err)
	}

	parseErrors := make([]error, 0)

	err := p.ParseMessage()
	if err == nil {
		return nil
	}
	parseErrors = append(parseErrors, err)

	err = p.ParseKvitto()
	if err == nil {
		return nil
	}
	parseErrors = append(parseErrors, err)

	errorsText := "Failed to parse as known format:\n"
	for _, v := range parseErrors {
		errorsText += v.Error() + "\n"
	}
	return errors.New(errorsText)
}

func (p *BongParser) Reset() {
	p.currentTokenIndex = 0
}

func (p *BongParser) ReadToken() *TokenBlock {
	token := p.PeekToken()
	p.DiscardToken()
	return token
}

func (p *BongParser) ReadTrimmed() string {
	return strings.TrimSpace(p.ReadToken().text)
}

func (p *BongParser) PeekToken() *TokenBlock {
	if len(p.tokenizer.tokenBlocks) <= p.currentTokenIndex {
		return nil
	}
	return &p.tokenizer.tokenBlocks[p.currentTokenIndex]
}
func (p *BongParser) IsAtEnd() bool {
	return len(p.tokenizer.tokenBlocks) <= p.currentTokenIndex
}

func (p *BongParser) DiscardToken() {
	p.currentTokenIndex++
}

func (p *BongParser) ExpectFormat(format string) bool {
	token := p.PeekToken()
	if token == nil {
		return false
	}
	if token.specialType != Text {
		return false
	}
	if token.format.shortForm() != format {
		return false
	}
	return true
}

func (p *BongParser) ExpectFormats(formats []string) bool {
	id := p.currentTokenIndex
	for _, v := range formats {
		if !p.ExpectFormat(v) {
			return false
		}
		p.DiscardToken()
	}
	p.currentTokenIndex = id
	return true
}

func (p *BongParser) ParseMessage() error {
	p.Reset()
	if !p.ExpectFormat("10B") {
		return fmt.Errorf("Expected Kvitto")
	}
	message := p.ReadTrimmed()

	nextToken := p.PeekToken()
	if nextToken == nil {
		return fmt.Errorf("Early EOF")
	}
	if nextToken.specialType != FeedNLines {
		return fmt.Errorf("Expected FeedNLines")
	}

	p.message.Message = message
	p.bongType = BongMessage
	return nil
}

var (
	RegexTargetPrinter = regexp.MustCompile("TILL ([a-zA-ZåäöÅÄÖ]+)")
	RegexSoldCount     = regexp.MustCompile("([0-9]+) st")
	RegexFooter        = regexp.MustCompile("Beställd av: ([a-z-A-Z]+)\nBeställd: ([0-9a-z :]+)")
)

func (p *BongParser) ParseKvitto() error {
	p.Reset()
	if err := p.ParseHeader(); err != nil {
		return fmt.Errorf("Head parse error %w", err)
	}

	if err := p.ParseBody(); err != nil {
		return fmt.Errorf("Body parse error %w", err)
	}

	if err := p.ParseFooter(); err != nil {
		return fmt.Errorf("Footer parse error %w", err)
	}

	p.bongType = BongKvitto

	return nil
}

func (p *BongParser) ParseHeader() error {
	if !p.ExpectFormats([]string{"30", "20", "20U", "20", "10"}) {
		return errors.New("Wrong format")
	}

	token := p.ReadToken()
	if token == nil {
		println("A")
	}

	matches := RegexTargetPrinter.FindStringSubmatch(token.text)

	if len(matches) <= 1 {
		return errors.New("Could not find destination")
	}
	p.kvitto.TargetPrinter = matches[1]

	p.DiscardToken() // "Beställning"

	kvittoName := p.ReadTrimmed()
	switch kvittoName {
	case "KvittoNr.":
		p.kvitto.IsBong = false
	case "Nr.":
		p.kvitto.IsBong = true
	default:
		return fmt.Errorf("Invalid kvitto number prefiz: %s", kvittoName)
	}

	var err error
	p.kvitto.Number, err = strconv.Atoi(p.ReadTrimmed())
	if err != nil {
		return fmt.Errorf("Invalid kvitto number %w", err)
	}

	p.DiscardToken() // "-------------"

	return nil
}

func (p *BongParser) ParseBody() error {
	for {
		if p.IsAtEnd() {
			return nil
		}
		if p.ExpectFormat("0B") {
			return nil // Footer;
		}
		if p.ExpectFormats([]string{"10B", "10CBU"}) {
			// Category assignment
			p.DiscardToken() // "\n"
			p.currentCategory = p.ReadTrimmed()
		}
		if p.ExpectFormat("10B") {
			p.DiscardToken()
		}
		if !p.ExpectFormats([]string{"20B", "10B"}) {
			return errors.New("Wrong format")
		}

		matches := RegexSoldCount.FindStringSubmatch(p.ReadTrimmed())
		if len(matches) <= 1 {
			return errors.New("Cant find sold count")
		}
		var err error

		currentSold := SoldProduct{}
		currentSold.Category = p.currentCategory

		currentSold.Count, err = strconv.Atoi(matches[1])
		if err != nil {
			return err
		}
		currentSold.Name = p.ReadTrimmed()
		currentSold.ExtraText = ""

		if p.ExpectFormat("10B") {
			extra_text := p.ReadTrimmed()
			if extra_text != "-----------------------" {
				currentSold.ExtraText = extra_text
			}
		}
		p.kvitto.Sold = append(p.kvitto.Sold, currentSold)
	}
}

func (p *BongParser) ParseFooter() error {
	if !p.ExpectFormat("0B") {
		return errors.New("Wrong format")
	}

	matches := RegexFooter.FindStringSubmatch(p.ReadTrimmed())
	if len(matches) <= 2 {
		return errors.New("Cant find seller")
	}
	loc, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		return fmt.Errorf("unable to load location: %v", err)
	}

	p.kvitto.SoldBy = matches[1]
	date := monthReplacer.Replace(matches[2])
	t, err := time.ParseInLocation("_2 Jan 15:04", date, loc)
	if err != nil {
		return fmt.Errorf("unable to parse time: %v", err)
	}
	t = t.AddDate(time.Now().Year(), 0, 0)
	p.kvitto.Time = t
	return nil
}
