package network

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"

	"github.com/raszia/gotiny"
)

type Packet struct {
	T    byte   `json:"T,omitempty"`
	Data []byte `json:"Data,omitempty"`
	EF   byte   `json:"EF,omitempty"`
}

func (p *Packet) Bytes() []byte {
	// gob.Register(Packet{})
	// b := bytes.Buffer{}
	// e := gob.NewEncoder(&b)
	// err := e.Encode(p)
	// if err != nil {
	// 	fmt.Println(`failed gob Encode`, err)
	// }
	// return b.Bytes()

	// gotiny.NewEncoder()
	return gotiny.Marshal(p)

}

func FromBytes(data []byte) Packet {
	// 	gob.Register(Packet{})
	p := Packet{}
	json.Unmarshal(data, p)
	// 	b := bytes.Buffer{}
	// 	b.Write(data)
	// 	d := gob.NewDecoder(&b)
	// 	err := d.Decode(&p)
	// 	if err != nil {
	// 		fmt.Println(`failed gob Decode`, err)
	// 	}
	// 	return p
	return p
}

type SX map[string]interface{}

// go binary encoder
func ToGOB64(m SX) string {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(m)
	if err != nil {
		fmt.Println(`failed gob Encode`, err)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

// go binary decoder
func FromGOB64(str string) SX {
	m := SX{}
	by, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		fmt.Println(`failed base64 Decode`, err)
	}
	b := bytes.Buffer{}
	b.Write(by)
	d := gob.NewDecoder(&b)
	err = d.Decode(&m)
	if err != nil {
		fmt.Println(`failed gob Decode`, err)
	}
	return m
}
