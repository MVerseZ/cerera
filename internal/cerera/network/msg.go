package network

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const headerLength = 12

type HeaderMsg string

const (
	hJoin       HeaderMsg = "Join"
	hSync       HeaderMsg = "Sync"
	hRequest    HeaderMsg = "Request"
	hPrePrepare HeaderMsg = "PrePrepare"
	hPrepare    HeaderMsg = "Prepare"
	hCommit     HeaderMsg = "Commit"
	hReply      HeaderMsg = "Reply"
)

type Msg interface {
	String() string
}

// <JOIN, address, pubkey>
type JoinMsg struct {
	Operation string  `json:"operation"`
	Timestamp int     `json:"timestamp"`
	ClientID  int     `json:"clientID"`
	CRequest  Request `json:"request"`
	RAddr     string  `json:"address"`
}

func (msg JoinMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

// <SYNC, address>
type SyncMsg struct {
	Operation string  `json:"operation"`
	Timestamp int     `json:"timestamp"`
	ClientID  int     `json:"clientID"`
	CRequest  Request `json:"request"`
}

func (msg SyncMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

// <REQUEST, o, t, c>
type RequestMsg struct {
	Operation string  `json:"operation"`
	Timestamp int     `json:"timestamp"`
	ClientID  int     `json:"clientID"`
	CRequest  Request `json:"request"`
}

func (msg RequestMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

// <<PRE-PREPARE,v,n,d>,m>
type PrePrepareMsg struct {
	Request    RequestMsg `json:"request"`
	Digest     string     `json:"digest"`
	ViewID     int        `json:"viewID"`
	SequenceID int        `json:"sequenceID"`
}

func (msg PrePrepareMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

// <PREPARE, v, n, d, i>
type PrepareMsg struct {
	Digest     string `json:"digest"`
	ViewID     int    `json:"viewID"`
	SequenceID int    `json:"sequenceID"`
	NodeID     int    `json:"nodeid"`
}

func (msg PrepareMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

// <COMMIT, v, n, d, i>
type CommitMsg struct {
	Digest     string `json:"digest"`
	ViewID     int    `json:"viewID"`
	SequenceID int    `json:"sequenceID"`
	NodeID     int    `json:"nodeid"`
}

func (msg CommitMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

// <REPLY, v, t, c, i, r>
type ReplyMsg struct {
	ViewID    int    `json:"viewID"`
	Timestamp int    `json:"timestamp"`
	ClientID  int    `json:"clientID"`
	NodeID    int    `json:"nodeid"`
	Result    string `json:"result"`
}

func (msg ReplyMsg) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

type Request struct {
	Message string `json:"message"`
	Digest  string `json:"digest"`
}

func (msg Request) String() string {
	bmsg, _ := json.MarshalIndent(msg, "", "	")
	return string(bmsg) + "\n"
}

func ComposeMsg(header HeaderMsg, payload interface{}, sig []byte) []byte {
	var bpayload []byte
	var err error
	t := reflect.TypeOf(payload)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		bpayload, err = json.Marshal(payload)
		if err != nil {
			panic(err)
		}
	case reflect.Slice:
		bpayload = payload.([]byte)
	default:
		panic(fmt.Errorf("not support type"))
	}

	b := make([]byte, headerLength)
	for i, h := range []byte(header) {
		b[i] = h
	}
	res := make([]byte, headerLength+len(bpayload)+len(sig))
	copy(res[:headerLength], b)
	copy(res[headerLength:], bpayload)
	fmt.Printf("SIG_LEN: %d\r\n", len(sig))
	if len(sig) > 0 {
		copy(res[headerLength+len(bpayload):], sig)
	}
	return res
}

func SplitMsg(bmsg []byte) (HeaderMsg, []byte, []byte) {
	var header HeaderMsg
	var payload []byte
	var signature []byte
	hbyte := bmsg[:headerLength]
	hhbyte := make([]byte, 0)
	for _, h := range hbyte {
		if h != byte(0) {
			hhbyte = append(hhbyte, h)
		}
	}
	header = HeaderMsg(hhbyte)
	switch header {
	case hRequest, hPrePrepare, hPrepare, hCommit:
		payload = bmsg[headerLength : len(bmsg)-71] //256
		signature = bmsg[len(bmsg)-71:]
	case hReply:
		payload = bmsg[headerLength:]
		signature = []byte{}
	case hJoin:
		payload = bmsg[headerLength : len(bmsg)-71] //256
		signature = bmsg[len(bmsg)-71:]

		// li := strings.LastIndex(string(bmsg), "}}") + 2
		// payload = bmsg[headerLength:li]
		// signature = bmsg[li:]

		fmt.Printf("Join msg:%s\r\n", bmsg)
		fmt.Printf("Join msg payload:%s\r\n", payload)
		fmt.Printf("Join msg sig 1:%s\r\n", signature)
	case hSync:
		li := strings.LastIndex(string(bmsg), "}}") + 2
		payload = bmsg[headerLength:li]
		signature = bmsg[li:]

		fmt.Printf("Sync msg:%s\r\n", bmsg)
		fmt.Printf("Sync msg payload:%s\r\n", payload)
		fmt.Printf("Sync msg sig 1:%s\r\n", signature)
	}
	return header, payload, signature
}

func printMsgLog(msg Msg) {
	fmt.Println(msg.String())
}

func logHandleMsg(header HeaderMsg, msg Msg, from int) {
	fmt.Printf("Receive %s msg from localhost:%d\n", header, nodeIdToPort(from))
	printMsgLog(msg)
}

func logBroadcastMsg(header HeaderMsg, msg Msg) {
	fmt.Printf("send/broadcast %s msg \n", header)
	printMsgLog(msg)
}
