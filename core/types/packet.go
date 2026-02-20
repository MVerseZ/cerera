package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"

	"github.com/cerera/internal/cerera/common"
)

func CreateVavilovEvent(method string, status byte, topic string) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = method
	packetData.status = status
	packetData.Topic = topic
	packetData.Type = "EVENT"
	return packetData
}

func CreateVavilovPacket2(method string, from Address, status byte, topic string) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = method
	packetData.status = status
	packetData.Topic = topic
	if !from.IsEmpty() {
		packetData.addr = from
	}
	return packetData
}

func CreateVavilovPacket3(method string, status byte, topic string) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = method
	packetData.status = status
	packetData.Topic = topic
	return packetData
}

func CreateValidatePacket(pk []byte) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = "vavilov_chelomei"
	packetData.status = 0x1
	packetData.pk = pk
	return packetData
}

func CreateVavilovNamedPacketWithAddr(pk *ecdsa.PublicKey, method string) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = method
	packetData.addr = PubkeyToAddress(*pk)
	packetData.status = 0xf

	packetData.pk = elliptic.Marshal(pk.Curve, pk.X, pk.Y)
	return packetData
}

func CreateVavilovAccountPacket(pk ecdsa.PublicKey) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = "vavilov.account.status"
	packetData.status = 0x1
	packetData.Topic = "ACCOUNT STATUS"
	addr := PubkeyToAddress(pk)
	dpKey := elliptic.Marshal(pk.Curve, pk.X, pk.Y)
	packetData.SetData(dpKey, addr)
	return packetData
}

func CreateVavilovPacketInnerPacket(adr Address) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = "vavilov_0x0"
	packetData.status = 0xf7
	packetData.Topic = "MINE__BLOCK__"
	packetData.addr = adr
	return packetData
}

func CreateResponcePacket(method string, data []byte) PacketData {
	packetData := &VavilovPacketData{}
	packetData.method = method
	packetData.status = 0xac
	packetData.Topic = "RPC_RESPONCE"
	packetData.data = data
	return packetData
}

type PacketData interface {
	Method() string
	From() Address
	Status() byte
	IncStatus()
	SetNamed(addr Address)
	SetBalance(b *big.Int)
	GetBalance() *big.Int
	GetPK() []byte
	SetData(pubKey []byte, addr Address)
	SetTx(tx *GTransaction)
	GetTx() *GTransaction
	GetSize() int
	SetSize(s int)
	SetSnap(map[string]interface{})
	GetSnap() map[string]interface{}
	ChangeStatus(status byte) byte
	// type of packet (EVENT, QUERY)
	GetType() string
	GetBinaryType() byte
	// txs block others
	SetPoolSnap(map[string]GTransactions)
	// set packet data
	GetEventPayload() []byte
	// hmmm
	SetLatest(hash common.Hash, index int, timestamp string)
	GetLatest() LatestBlock
}

type LatestBlock struct {
	Index int
	Hash  common.Hash
	Time  string
}

type VavilovPacketData struct {
	Topic string
	// inner method
	method string
	// address from (sender)
	addr Address
	// status of packet
	status      byte
	pk          []byte
	VREZ        big.Int
	Balance     *big.Int
	Transaction *GTransaction
	data        []byte
	size        int
	snap        map[string]interface{}
	// type of packet (EVENT, QUERY)
	Type string
	// txs block others
	PoolSnapshot map[string]GTransactions
	// hmmmm
	Block LatestBlock
}

func (v *VavilovPacketData) SetSize(sz int) {
	v.size = sz
}

func (v *VavilovPacketData) GetSize() int {
	return v.size
}

func (v *VavilovPacketData) SetTx(tx *GTransaction) {
	v.Transaction = tx
}

func (v *VavilovPacketData) GetTx() *GTransaction {
	return v.Transaction
}

func (v *VavilovPacketData) Method() string {
	return v.method
}

func (v *VavilovPacketData) From() Address {
	return v.addr
}

func (v *VavilovPacketData) Status() byte {
	return v.status
}

func (v *VavilovPacketData) IncStatus() {
	if v.status == 0xf7 {
		v.status = 0xf8
	} else {
		v.status = 0x2
	}
}

func (v *VavilovPacketData) SetNamed(addr Address) {
	v.addr = addr
}

func (v *VavilovPacketData) SetBalance(b *big.Int) {
	v.Balance = b
}

func (v *VavilovPacketData) GetPK() []byte {
	return v.pk
}

func (v *VavilovPacketData) SetData(pk []byte, address Address) {
	v.pk = pk
	v.addr = address
}

func (v *VavilovPacketData) GetBalance() *big.Int {
	return v.Balance
}

func (v *VavilovPacketData) SetSnap(snap map[string]interface{}) {
	v.snap = snap
}

func (v *VavilovPacketData) GetSnap() map[string]interface{} {
	return v.snap
}

func (v *VavilovPacketData) ChangeStatus(status byte) byte {
	v.status = status
	return v.status
}

// type of packet (EVENT, QUERY)

func (v *VavilovPacketData) GetType() string {
	return v.Type
}

func (v *VavilovPacketData) GetBinaryType() byte {
	if v.Type == "EVENT" {
		return 0x1
	} else {
		return 0x2
	}
}

func (v *VavilovPacketData) SetPoolSnap(snap map[string]GTransactions) {
	v.PoolSnapshot = snap
}

func (v *VavilovPacketData) GetEventPayload() []byte {
	return v.data
}

func (v *VavilovPacketData) SetLatest(h common.Hash, i int, t string) {
	v.Block.Hash = h
	v.Block.Index = i
	v.Block.Time = t
}

func (v *VavilovPacketData) GetLatest() LatestBlock {
	return v.Block
}

type DataEvent struct {
	Data  PacketData
	Topic string
	Type  string
}

func (de *DataEvent) GetMethod() string {
	return de.Data.Method()
}

func (de *DataEvent) GetPK() []byte {
	return de.Data.GetPK()
}

func (de *DataEvent) GetParams() []byte {
	return de.Data.GetPK()
}

func (de *DataEvent) GetBalance() *big.Int {
	return de.Data.GetBalance()
}

func (de *DataEvent) GetType() string {
	return de.Type
}
