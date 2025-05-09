package network

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
	"golang.org/x/crypto/blake2b"
)

const ViewID = 0

type KnownNode struct {
	nodeID types.Address
	url    string
	// pubkey *rsa.PublicKey
	pubkey *ecdsa.PublicKey
}

type PreKnownNode struct {
	nodeID int
	url    string
	// pubkey *rsa.PublicKey
	pubkey     *ecdsa.PublicKey
	connection net.Conn
}

type Node struct {
	NodeID types.Address
	// BFT
	preKnownNodes []*PreKnownNode
	knownNodes    []*KnownNode
	clientNode    *KnownNode
	sequenceID    int
	View          int
	msgQueue      chan []byte
	keypair       Keypair
	msgLog        *MsgLog
	requestPool   map[string]*RequestMsg
	// STUFF
	mutex     sync.Mutex
	syncQueue map[types.Address]int
}

type Keypair struct {
	privkey *ecdsa.PrivateKey
	pubkey  *ecdsa.PublicKey
}

type MsgLog struct {
	preprepareLog map[string]map[types.Address]bool
	prepareLog    map[string]map[types.Address]bool
	commitLog     map[string]map[types.Address]bool
	replyLog      map[string]bool
}

func NewNode(cfg *config.Config) *Node {
	publicKey, err := types.DecodeByteToPublicKey(cfg.NetCfg.PUB)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return &Node{
		cfg.NetCfg.ADDR,
		make([]*PreKnownNode, 0),
		make([]*KnownNode, 0), // first there is no known nodes.
		nil,                   // known node
		0,
		ViewID,
		make(chan []byte),
		Keypair{
			privkey: types.DecodePrivKey(cfg.NetCfg.PRIV),
			pubkey:  publicKey,
		},
		&MsgLog{
			make(map[string]map[types.Address]bool),
			make(map[string]map[types.Address]bool),
			make(map[string]map[types.Address]bool),
			make(map[string]bool),
		},
		make(map[string]*RequestMsg),
		sync.Mutex{},
		make(map[types.Address]int),
	}
}

func (node *Node) getSequenceID() int {
	seq := node.sequenceID
	node.sequenceID++
	return seq
}

func (node *Node) Start() {
	go node.handleMsg()
}

func (node *Node) handleMsg() {
	for {
		msg := <-node.msgQueue
		header, payload, sign := SplitMsg(msg)
		switch header {
		case hJoin:
			node.handleJoin(payload, sign)
		case hSync:
			node.handleSync(payload, sign)
		case hRequest:
			node.handleRequest(payload, sign)
		case hReplySync:
			node.handleReplySync(payload, sign)
		case hSyncDone:
			node.handleSyncDone(payload, sign)
		case hTx:
			node.handleInputTx(payload, sign)
		case hAccOp:
			node.handleInputAcc(payload, sign)
			// case hPrePrepare:
			// 	node.handlePrePrepare(payload, sign)
			// case hPrepare:
			// 	node.handlePrepare(payload, sign)
			// case hCommit:
			// 	node.handleCommit(payload, sign)
		}
	}
}

func (node *Node) handleJoin(payload []byte, sig []byte) {
	fmt.Println("Join message received")
	// time.Sleep(1 * time.Second)

	fmt.Println(node.knownNodes)
	fmt.Println(node.preKnownNodes)
	fmt.Println(peers)

	var joinMsg JoinMsg
	err := json.Unmarshal(payload, &joinMsg)
	if err != nil {
		fmt.Printf("error happened while JSON unmarshal:%v", err)
		return
	}

	var v = storage.GetVault()

	node.syncQueue[joinMsg.ClientID] = v.GetCount()
	var clientSABytes = joinMsg.StateAccount
	var clientSA = types.BytesToStateAccount(clientSABytes)
	v.Put(clientSA.Address, clientSA)

	var msg = node.sequenceID
	req := Request{
		string(rune(msg)),
		hex.EncodeToString(generateDigest(msg)),
	}

	var nodeSA = v.GetOwner()
	var nodeSABytes = nodeSA.Bytes()
	reqmsg := &SyncMsg{
		"sync",
		int(time.Now().Unix()),
		joinMsg.ClientID,
		req,
		nodeSABytes,
	}

	Broadcast(ComposeMsg(hSync, reqmsg, []byte{}))

	// TODO CLIENT -> SERVER -> <HERE> -> CLIENT

	// var vlt = storage.GetVault()
	// fmt.Printf("preknown nodes: %d\r\n", len(node.preKnownNodes))
	// fmt.Printf("known nodes: %d\r\n", len(node.knownNodes))

	// msg := vlt.Size()

	// pbk := node.keypair.pubkey
	// b := types.EncodePublicKeyToByte(pbk)
	// var msg = string(b)
	// fmt.Printf("Send key: %s\r\n", msg)

	// req := Request{
	// 	msg,
	// 	hex.EncodeToString(generateDigest(msg)),
	// }
	// reqmsg := &SyncMsg{
	// 	"sync",
	// 	int(time.Now().Unix()),
	// 	node.NodeID,
	// 	req,
	// }
	// sig, err := signMessage(reqmsg, node.keypair.privkey)
	// if err != nil {
	// 	fmt.Printf("%v\n", err)
	// }

	// var joinMsg JoinMsg
	// err = json.Unmarshal(payload, &joinMsg)
	// if err != nil {
	// 	fmt.Printf("error happened while JSON unmarshal:%v", err)
	// 	return
	// }
	// remotePubKey, err := types.PublicKeyFromString(joinMsg.CRequest.Message)
	// if err != nil {
	// 	fmt.Printf("error happened while decode:%v", err)
	// 	panic(err)
	// }
	// fmt.Printf("Remote address for broadcast: %s\r\n", joinMsg.RAddr)

	// var newKnownNode = &KnownNode{
	// 	joinMsg.ClientID,
	// 	joinMsg.RAddr,
	// 	remotePubKey,
	// }
	// node.mutex.Lock()
	// defer node.mutex.Unlock()
	// node.knownNodes = append(node.knownNodes, newKnownNode)
	// node.broadcast(ComposeMsg(hSync, reqmsg, sig))
}

func (node *Node) handleSync(payload []byte, sig []byte) {
	// fmt.Println("Sync message received")
	// time.Sleep(1 * time.Second)

	var syncMsg SyncMsg
	err := json.Unmarshal(payload, &syncMsg)
	if err != nil {
		fmt.Printf("error happened while JSON unmarshal:%v", err)
		return
	}

	var vlt = storage.GetVault()

	vlt.Sync(syncMsg.SyncSA)
	var ssa = vlt.GetOwner()
	var ssab = ssa.Bytes()
	if syncMsg.ClientID == node.NodeID {
		reqmsg := &ReplySync{
			int(time.Now().Unix()),
			node.NodeID,
			0x1,
			ssab,
		}
		var sas = types.BytesToStateAccount(syncMsg.SyncSA)
		fmt.Println(sas.Address)

		Broadcast(ComposeMsg(hReplySync, reqmsg, []byte{}))
	}
	// node.syncQueue[sy]

	// logHandleMsg(hJoin, syncMsg.CRequest, syncMsg.ClientID)

	// var msg = node.sequenceID
	// req := Request{
	// 	node.NodeID.String(),
	// 	hex.EncodeToString(generateDigest(msg)),
	// }
	// reqmsg := &SyncMsg{
	// 	"sync",
	// 	int(time.Now().Unix()),
	// 	// node.NodeID,
	// 	req,
	// }

	// Broadcast(ComposeMsg(hSync, reqmsg, []byte{}))

	// fmt.Printf("preknown nodes: %d\r\n", len(node.preKnownNodes))
	// fmt.Printf("known nodes: %d\r\n", len(node.knownNodes))

	// var request SyncMsg
	// err = json.Unmarshal(payload, &request)

	// if err != nil {
	// 	fmt.Printf("error happened:%v", err)
	// 	return
	// }

	// logHandleMsg(hRequest, request, request.ClientID)
	// verify request's digest
	// vdig := verifyDigest(request.CRequest.Message, request.CRequest.Digest)
	// if vdig == false {
	// 	fmt.Printf("verifyDigest failed\n")
	// 	return
	// }
	// //verigy request's signature
	// remotePubKey, err := types.PublicKeyFromString(request.CRequest.Message)
	// if err != nil {
	// 	fmt.Printf("error happened while decode:%v", err)
	// 	panic(err)
	// }
	// // pks, err := types.PublicKeyToString(remotePubKey)
	// // if err != nil {
	// // 	fmt.Printf("error happened while key to string conv:%v", err)
	// // 	panic(err)
	// // }
	// _, err = verifySignatrue(request, sig, remotePubKey)
	// if err != nil {
	// 	fmt.Printf("verify signature failed:%v\n", err)
	// 	return
	// }

	// pbk := node.keypair.pubkey
	// msg, err := types.PublicKeyToString(pbk)
	// if err != nil {
	// 	panic(err)
	// }
	// // fmt.Printf("Send key: %s\r\n", msg)

	// req := Request{
	// 	msg,
	// 	hex.EncodeToString(generateDigest(msg)),
	// }
	// reqmsg := &SyncMsg{
	// 	"sync",
	// 	int(time.Now().Unix()),
	// 	node.NodeID,
	// 	req,
	// }
	// sig, err = signMessage(reqmsg, node.keypair.privkey)
	// if err != nil {
	// 	fmt.Printf("%v\n", err)
	// }
	// // logBroadcastMsg(hSync, reqmsg)
	// node.broadcast(ComposeMsg(hSync, reqmsg, sig))
}

func (node *Node) handleReplySync(payload []byte, sig []byte) {
	// fmt.Println("Reply sync message received")
	// time.Sleep(1 * time.Second)

	var replySync ReplySync
	err := json.Unmarshal(payload, &replySync)
	if err != nil {
		fmt.Printf("error happened while JSON unmarshal:%v", err)
		return
	}

	var vlt = storage.GetVault()
	var acc = vlt.GetPos(int64(node.syncQueue[replySync.ClientID]))

	node.syncQueue[replySync.ClientID] -= 1
	if node.syncQueue[replySync.ClientID] > 0 {
		var msg = node.sequenceID
		req := Request{
			string(rune(msg)),
			hex.EncodeToString(generateDigest(msg)),
		}
		reqmsg := &SyncMsg{
			"sync",
			int(time.Now().Unix()),
			replySync.ClientID,
			req,
			acc.Bytes(),
		}
		fmt.Println(replySync.ClientID)
		Broadcast(ComposeMsg(hSync, reqmsg, []byte{}))
	} else {
		reqmsg := &SyncDone{
			int(time.Now().Unix()),
			"DONE",
		}
		Broadcast(ComposeMsg(hSyncDone, reqmsg, []byte{}))
	}

}

func (node *Node) handleSyncDone(payload []byte, sig []byte) {
	fmt.Println("Sync done message received")
	// time.Sleep(1 * time.Second)

}

func (node *Node) handleRequest(payload []byte, sig []byte) {
	var request RequestMsg
	var prePrepareMsg PrePrepareMsg
	fmt.Println(payload)
	err := json.Unmarshal(payload, &request)

	if err != nil {
		fmt.Printf("error happened:%v", err)
		return
	}
	logHandleMsg(hRequest, request, request.ClientID)
	// verify request's digest
	vdig := verifyDigest(request.CRequest.Message, request.CRequest.Digest)
	if vdig == false {
		fmt.Printf("verifyDigest failed\n")
		return
	}
	//verigy request's signature
	_, err = verifySignatrue(request, sig, node.clientNode.pubkey)
	if err != nil {
		fmt.Printf("verify signature failed:%v\n", err)
		return
	}
	node.mutex.Lock()
	node.requestPool[request.CRequest.Digest] = &request
	seqID := node.getSequenceID()
	node.mutex.Unlock()
	prePrepareMsg = PrePrepareMsg{
		request,
		request.CRequest.Digest,
		ViewID,
		seqID,
	}
	//sign prePrepareMsg
	msgSig, err := node.signMessage(prePrepareMsg)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	msg := ComposeMsg(hPrePrepare, prePrepareMsg, msgSig)
	node.mutex.Lock()
	// put preprepare msg into log
	if node.msgLog.preprepareLog[prePrepareMsg.Digest] == nil {
		node.msgLog.preprepareLog[prePrepareMsg.Digest] = make(map[types.Address]bool)
	}
	node.msgLog.preprepareLog[prePrepareMsg.Digest][node.NodeID] = true
	node.mutex.Unlock()
	logBroadcastMsg(hPrePrepare, prePrepareMsg)
	node.broadcast(msg)
}

func (node *Node) handleInputAcc(payload []byte, sig []byte) {
	var accMsg AccMsg
	err := json.Unmarshal(payload, &accMsg)
	if err != nil {
		fmt.Printf("error happened while JSON unmarshal:%v", err)
		return
	}
	var vlt = storage.GetVault()
	vlt.Sync(accMsg.Data)
}

func (node *Node) handleInputTx(payload []byte, sig []byte) {
	var txMsg TxMsg
	err := json.Unmarshal(payload, &txMsg)
	if err != nil {
		fmt.Printf("error happened while JSON unmarshal:%v", err)
		return
	}
	var p = pool.Get()
	p.Funnel <- []*types.GTransaction{&txMsg.Data}
}

// func (node *Node) handlePrePrepare(payload []byte, sig []byte) {
// 	var prePrepareMsg PrePrepareMsg
// 	err := json.Unmarshal(payload, &prePrepareMsg)
// 	if err != nil {
// 		fmt.Printf("error happened:%v", err)
// 		return
// 	}
// 	pnodeId := node.findPrimaryNode()
// 	logHandleMsg(hPrePrepare, prePrepareMsg, pnodeId)
// 	msgPubkey := node.findNodePubkey(pnodeId)
// 	if msgPubkey == nil {
// 		fmt.Println("can't find primary node's public key")
// 		return
// 	}
// 	// verify msg's signature
// 	_, err = verifySignatrue(prePrepareMsg, sig, msgPubkey)
// 	if err != nil {
// 		fmt.Printf("verify signature failed:%v\n", err)
// 		return
// 	}

// 	// verify prePrepare's digest is equal to request's digest
// 	if prePrepareMsg.Digest != prePrepareMsg.Request.CRequest.Digest {
// 		fmt.Printf("verify digest failed\n")
// 		return
// 	}
// 	node.mutex.Lock()
// 	node.requestPool[prePrepareMsg.Request.CRequest.Digest] = &prePrepareMsg.Request
// 	node.mutex.Unlock()
// 	err = node.verifyRequestDigest(prePrepareMsg.Digest)
// 	if err != nil {
// 		fmt.Printf("%v\n", err)
// 		return
// 	}
// 	// put preprepare's msg into log
// 	node.mutex.Lock()
// 	if node.msgLog.preprepareLog[prePrepareMsg.Digest] == nil {
// 		node.msgLog.preprepareLog[prePrepareMsg.Digest] = make(map[types.Address]bool)
// 	}
// 	node.msgLog.preprepareLog[prePrepareMsg.Digest][pnodeId] = true
// 	node.mutex.Unlock()
// 	prepareMsg := PrepareMsg{
// 		prePrepareMsg.Digest,
// 		ViewID,
// 		prePrepareMsg.SequenceID,
// 		1,//node.NodeID,
// 	}
// 	// sign prepare msg
// 	msgSig, err := signMessage(prepareMsg, node.keypair.privkey)
// 	if err != nil {
// 		fmt.Printf("%v\n", err)
// 		return
// 	}
// 	sendMsg := ComposeMsg(hPrepare, prepareMsg, msgSig)
// 	node.mutex.Lock()
// 	// put prepare msg into log
// 	if node.msgLog.prepareLog[prepareMsg.Digest] == nil {
// 		node.msgLog.prepareLog[prepareMsg.Digest] = make(map[types.Address]bool)
// 	}
// 	node.msgLog.prepareLog[prepareMsg.Digest][node.NodeID] = true
// 	node.mutex.Unlock()
// 	logBroadcastMsg(hPrepare, prepareMsg)
// 	node.broadcast(sendMsg)
// }

// func (node *Node) handlePrepare(payload []byte, sig []byte) {
// 	var prepareMsg PrepareMsg
// 	err := json.Unmarshal(payload, &prepareMsg)
// 	if err != nil {
// 		fmt.Printf("error happened:%v", err)
// 		return
// 	}
// 	logHandleMsg(hPrepare, prepareMsg, prepareMsg.NodeID)
// 	// verify prepareMsg
// 	pubkey := node.findNodePubkey(prepareMsg.NodeID)
// 	_, err = verifySignatrue(prepareMsg, sig, pubkey)
// 	if err != nil {
// 		fmt.Printf("verify signature failed:%v\n", err)
// 		return
// 	}
// 	// verify request's digest
// 	err = node.verifyRequestDigest(prepareMsg.Digest)
// 	if err != nil {
// 		fmt.Printf("%v\n", err)
// 		return
// 	}
// 	// verify prepareMsg's digest is equal to preprepareMsg's digest
// 	pnodeId := node.findPrimaryNode()
// 	exist := node.msgLog.preprepareLog[prepareMsg.Digest][pnodeId]
// 	if !exist {
// 		fmt.Printf("this digest's preprepare msg by %d not existed\n", pnodeId)
// 		return
// 	}
// 	// put prepareMsg into log
// 	node.mutex.Lock()
// 	if node.msgLog.prepareLog[prepareMsg.Digest] == nil {
// 		node.msgLog.prepareLog[prepareMsg.Digest] = make(map[int]bool)
// 	}
// 	node.msgLog.prepareLog[prepareMsg.Digest][prepareMsg.NodeID] = true
// 	node.mutex.Unlock()
// 	// if receive prepare msg >= 2f +1, then broadcast commit msg
// 	limit := node.countNeedReceiveMsgAmount()
// 	sum, err := node.findVerifiedPrepareMsgCount(prepareMsg.Digest)
// 	if err != nil {
// 		fmt.Printf("error happened:%v", err)
// 		return
// 	}
// 	if sum >= limit {
// 		// if already send commit msg, then do nothing
// 		node.mutex.Lock()
// 		exist, _ := node.msgLog.commitLog[prepareMsg.Digest][node.NodeID]
// 		node.mutex.Unlock()
// 		if exist != false {
// 			return
// 		}
// 		//send commit msg
// 		commitMsg := CommitMsg{
// 			prepareMsg.Digest,
// 			prepareMsg.ViewID,
// 			prepareMsg.SequenceID,
// 			node.NodeID,
// 		}
// 		sig, err := node.signMessage(commitMsg)
// 		if err != nil {
// 			fmt.Printf("sign message happened error:%v\n", err)
// 		}
// 		sendMsg := ComposeMsg(hCommit, commitMsg, sig)
// 		// put commit msg to log
// 		node.mutex.Lock()
// 		if node.msgLog.commitLog[commitMsg.Digest] == nil {
// 			node.msgLog.commitLog[commitMsg.Digest] = make(map[int]bool)
// 		}
// 		node.msgLog.commitLog[commitMsg.Digest][node.NodeID] = true
// 		node.mutex.Unlock()
// 		logBroadcastMsg(hCommit, commitMsg)
// 		node.broadcast(sendMsg)
// 	}
// }

// func (node *Node) handleCommit(payload []byte, sig []byte) {
// 	var commitMsg CommitMsg
// 	err := json.Unmarshal(payload, &commitMsg)
// 	if err != nil {
// 		fmt.Printf("error happened:%v", err)
// 	}
// 	logHandleMsg(hCommit, commitMsg, commitMsg.NodeID)
// 	//verify commitMsg's signature
// 	msgPubKey := node.findNodePubkey(commitMsg.NodeID)
// 	verify, err := verifySignatrue(commitMsg, sig, msgPubKey)
// 	if err != nil {
// 		fmt.Printf("verify signature failed:%v\n", err)
// 		return
// 	}
// 	if verify == false {
// 		fmt.Printf("verify signature failed\n")
// 		return
// 	}
// 	// verify request's digest
// 	err = node.verifyRequestDigest(commitMsg.Digest)
// 	if err != nil {
// 		fmt.Printf("%v\n", err)
// 		return
// 	}
// 	// put commitMsg into log
// 	node.mutex.Lock()
// 	if node.msgLog.commitLog[commitMsg.Digest] == nil {
// 		node.msgLog.commitLog[commitMsg.Digest] = make(map[int]bool)
// 	}
// 	node.msgLog.commitLog[commitMsg.Digest][commitMsg.NodeID] = true
// 	node.mutex.Unlock()
// 	// if receive commit msg >= 2f +1, then send reply msg to client
// 	limit := node.countNeedReceiveMsgAmount()
// 	sum, err := node.findVerifiedCommitMsgCount(commitMsg.Digest)
// 	if err != nil {
// 		fmt.Printf("error happened:%v", err)
// 		return
// 	}
// 	if sum >= limit {
// 		// if already send reply msg, then do nothing
// 		node.mutex.Lock()
// 		exist := node.msgLog.replyLog[commitMsg.Digest]
// 		node.mutex.Unlock()
// 		if exist == true {
// 			return
// 		}
// 		// send reply msg
// 		node.mutex.Lock()
// 		requestMsg := node.requestPool[commitMsg.Digest]
// 		node.mutex.Unlock()
// 		fmt.Printf("operstion:%s  message:%s executed... \n", requestMsg.Operation, requestMsg.CRequest.Message)
// 		done := fmt.Sprintf("operstion:%s  message:%s done ", requestMsg.Operation, requestMsg.CRequest.Message)
// 		replyMsg := ReplyMsg{
// 			node.View,
// 			int(time.Now().Unix()),
// 			requestMsg.ClientID,
// 			node.NodeID,
// 			done,
// 		}
// 		logBroadcastMsg(hReply, replyMsg)
// 		send(ComposeMsg(hReply, replyMsg, []byte{}), node.clientNode.url)
// 		node.mutex.Lock()
// 		node.msgLog.replyLog[commitMsg.Digest] = true
// 		node.mutex.Unlock()
// 	}
// }

func (node *Node) verifyRequestDigest(digest string) error {
	node.mutex.Lock()
	_, ok := node.requestPool[digest]
	if !ok {
		node.mutex.Unlock()
		return fmt.Errorf("verify request digest failed\n")

	}
	node.mutex.Unlock()
	return nil
}

func (node *Node) findVerifiedPrepareMsgCount(digest string) (int, error) {
	sum := 0
	node.mutex.Lock()
	for _, exist := range node.msgLog.prepareLog[digest] {
		if exist == true {
			sum++
		}
	}
	node.mutex.Unlock()
	return sum, nil
}

func (node *Node) findVerifiedCommitMsgCount(digest string) (int, error) {
	sum := 0
	node.mutex.Lock()
	for _, exist := range node.msgLog.commitLog[digest] {

		if exist == true {
			sum++
		}
	}
	node.mutex.Unlock()
	return sum, nil
}

func (node *Node) broadcast(data []byte) {
	for _, knownNode := range node.knownNodes {
		// fmt.Printf("Compare known node ID %d, with local ID %d\r\n", knownNode.nodeID, node.NodeID)
		if knownNode.nodeID != node.NodeID {
			fmt.Printf("Send to %s, with ID %d\r\n", knownNode.url, knownNode.nodeID)
			err := send(data, knownNode.url)
			if err != nil {
				fmt.Printf("%v", err)
			}
		}
	}

}

func (node *Node) BroadcastAcc(acc types.StateAccount) {
	accBytes := acc.Bytes()
	msg := &AccMsg{
		int(time.Now().Unix()),
		node.NodeID,
		accBytes,
	}
	Broadcast(ComposeMsg(hAccOp, msg, []byte{}))
}

func (node *Node) BroadcastTx(tx types.GTransaction) {
	msg := &TxMsg{
		int(time.Now().Unix()),
		node.NodeID,
		tx,
	}
	Broadcast(ComposeMsg(hTx, msg, []byte{}))
}

func (node *Node) findNodePubkey(nodeId types.Address) *ecdsa.PublicKey {
	for _, knownNode := range node.knownNodes {
		if knownNode.nodeID.Hex() == nodeId.Hex() {
			return knownNode.pubkey
		}
	}
	return nil
}

func (node *Node) signMessage(msg interface{}) ([]byte, error) {
	sig, err := signMessage(msg, node.keypair.privkey)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func send(data []byte, url string) error {
	conn, err := net.Dial("tcp", url)
	if err != nil {
		return fmt.Errorf("%s is not online", url)
	}
	defer conn.Close()
	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

func (node *Node) findPrimaryNode() int {
	return ViewID % len(node.knownNodes)
}

func (node *Node) countTolerateFaultNode() int {
	return (len(node.knownNodes) - 1) / 3
}

func (node *Node) countNeedReceiveMsgAmount() int {
	f := node.countTolerateFaultNode()
	return 2*f + 1
}

func generateDigest(msg interface{}) []byte {
	bmsg, _ := json.Marshal(msg)
	hash := blake2b.Sum256(bmsg)
	return hash[:]
}
func verifyDigest(msg interface{}, digest string) bool {
	return hex.EncodeToString(generateDigest(msg)) == digest
}
func verifySignatrue(msg interface{}, sig []byte, pubkey *ecdsa.PublicKey) (bool, error) {
	dig := generateDigest(msg)
	return ecdsa.VerifyASN1(pubkey, dig, sig), nil
	// err := rsa.VerifyPKCS1v15(pubkey, crypto.SHA256, dig, sig)
	// if err != nil {
	// 	return false, err
	// }
	// return true, nil
}
func signMessage(msg interface{}, privkey *ecdsa.PrivateKey) ([]byte, error) {
	dig := generateDigest(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, privkey, dig)
	// sig, err := rsa.SignPKCS1v15(rand.Reader, privkey, crypto.SHA256, dig)
	if err != nil {
		return nil, err
	}
	return sig, nil
}
