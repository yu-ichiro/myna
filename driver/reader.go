package driver

import (
	"os"
	"fmt"
	"time"
	_ "errors"
	"github.com/urfave/cli"
	"github.com/ebfe/scard"
)

type Reader struct {
	ctx *scard.Context
	c *cli.Context
	name string
	card *scard.Card
}

func NewReader(c *cli.Context) *Reader {
	ctx, err := scard.EstablishContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return nil
	}

	readers, err := ctx.ListReaders()
	if err != nil || len(readers) == 0 {
		fmt.Fprintf(os.Stderr, "エラー: リーダーが見つかりません。\n")
		return nil
	}
	if len(readers) >= 2 {
		fmt.Fprintf(os.Stderr,
			"警告: 複数のリーダーが見つかりました。最初のものを使います。\n")
	}

	reader := new(Reader)
	reader.ctx = ctx
	reader.c = c
	reader.name = readers[0]
	reader.card = nil
	return reader
}

func (self *Reader) Finalize() {
	self.ctx.Release()
}

func (self *Reader) GetCard() *scard.Card {
	card, _ := self.ctx.Connect(
		self.name, scard.ShareExclusive, scard.ProtocolAny)
	self.card = card
	return card
}

func (self *Reader) WaitForCard() *scard.Card {
	rs := make([]scard.ReaderState, 1)
	rs[0].Reader = self.name
	rs[0].CurrentState = scard.StateUnaware
	for i := 0; i < 3; i++ {
		err := self.ctx.GetStatusChange(rs, -1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %s\n", err)
			return nil
		}
		if rs[0].EventState&scard.StatePresent != 0 {
			card, _ := self.ctx.Connect(
				self.name, scard.ShareExclusive, scard.ProtocolAny)
			self.card = card
			return card
		}
		fmt.Fprintf(os.Stderr, "wait for card...\n")
		time.Sleep(1 * time.Second)
	}
	fmt.Fprintf(os.Stderr, "カードが見つかりません。\n")
	os.Exit(1)
	return nil
}

func (self *Reader) SelectAP(aid string) bool {
	sw1, sw2 := self.SelectDF(aid)
	if (sw1 == 0x90 && sw2 == 0x00) {
		return true
	} else {
		return false
	}

}

func (self *Reader) SelectDF(id string) (uint8, uint8) {
	bid := ToBytes(id)
	apdu := "00 A4 04 0C" + fmt.Sprintf(" %02X % X", len(bid), bid)
	sw1, sw2, _ := self.Tx(apdu)
	return sw1, sw2
}

func (self *Reader) SelectEF(id string) (uint8, uint8) {
	bid := ToBytes(id)
	apdu := fmt.Sprintf("00 A4 02 0C %02X % X", len(bid), bid)
	sw1, sw2, _ := self.Tx(apdu)
	return sw1, sw2
}

func (self *Reader) Verify(pin string) (uint8, uint8) {
	var apdu string
	bpin := []byte(pin)
	apdu = fmt.Sprintf("00 20 00 80 %02X % X", len(bpin), bpin)
	sw1, sw2, _ := self.Tx(apdu)
	return sw1, sw2
}

func (self *Reader) Tx(apdu string) (uint8, uint8, []byte) {
	card := self.card
	if self.c.Bool("verbose") {
		fmt.Printf("< %v\n", apdu)
	}
	cmd := ToBytes(apdu)
	res, err := card.Transmit(cmd)
	if err != nil {
		fmt.Printf("err: %s\n", err)
		return 0, 0, nil
	}

	if self.c.Bool("verbose") {
		for i := 0; i < len(res); i++ {
			if i % 0x10 == 0 {
				fmt.Print(">")
			}
			fmt.Printf(" %02X", res[i])
			if i % 0x10 == 0x0f {
				fmt.Println()
			}
		}
		fmt.Println()
	}

	l := len(res)
	if l == 2 {
		return res[0], res[1], nil
	}else if l > 2 {
		return res[l-2], res[l-1], res[:l-2]
	}
	return 0, 0, nil
}


func (self *Reader) ReadBinary(size uint16) []byte {
	var l uint8
	var apdu string
	var pos uint16
	pos = 0
	var res []byte

	for pos < size {
		if size - pos > 0xFF {
			l = 0
		}else{
			l = uint8(size - pos)
		}
		apdu = fmt.Sprintf("00 B0 %02X %02X %02X",
			pos >> 8 & 0xFF, pos & 0xFF, l)
		sw1, sw2, data := self.Tx(apdu)
		if sw1 != 0x90 || sw2 != 0x00 {
			return nil
		}
		res = append(res, data...)
		pos += uint16(len(data))
	}
	return res
}
