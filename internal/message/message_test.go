package message

import (
	"strings"
	"testing"
)

func TestMTypeAndHelpers(t *testing.T) {
	cases := []struct {
		mt   MType
		want string
	}{
		{MTPrePrepare, "PrePrepare"},
		{MTPrepare, "Prepare"},
		{MTCommit, "Commit"},
		{MTCheckpoint, "Checkpoint"},
		{MTRequest, "Unknown"},
		{MTViewChange, "Unknown"},
		{MTNewView, "Unknown"},
		{MType(99), "Unknown"},
	}
	for _, tc := range cases {
		if tc.mt.String() != tc.want {
			t.Fatalf("%d -> %s, want %s", tc.mt, tc.mt.String(), tc.want)
		}
	}
	if Digest(nil) != "0x111111" {
		t.Fatal(Digest(nil))
	}
	if PortByID(7) != 30007 {
		t.Fatal(PortByID(7))
	}
	if MaxFaultyNode != 1 || TotalNodeNO != 4 {
		t.Fatalf("faulty=%d total=%d", MaxFaultyNode, TotalNodeNO)
	}
}

func TestRequestAndConsensusMessage(t *testing.T) {
	req := &Request{ClientID: "client", TimeStamp: 9, Operation: "op", SeqID: 3}
	if !strings.Contains(req.String(), "client") || !strings.Contains(req.String(), "op") {
		t.Fatal(req.String())
	}
	msg := CreateConMsg(MTCommit, req)
	if msg == nil || msg.Typ != MTCommit || len(msg.Payload) == 0 || !msg.Verify() {
		t.Fatalf("%+v", msg)
	}
	if !strings.Contains(msg.String(), "Commit") {
		t.Fatal(msg.String())
	}
	if CreateConMsg(MTRequest, make(chan int)) != nil {
		t.Fatal("expected nil for unmarshalable payload")
	}

	vc := &ViewChange{NewViewID: 2, LastCPSeq: 4}
	if !strings.Contains(vc.Digest(), "2-4") {
		t.Fatal(vc.Digest())
	}
	if !(OMessage{}).EQ(nil) {
		t.Fatal("expected equal")
	}
	_ = Reply{Result: "ok"}
	_ = Prepare{NodeID: 1}
	_ = Commit{NodeID: 1}
	_ = CheckPoint{NodeID: 1}
	_ = PTuple{}
	_ = NewView{}
	_ = RequestRecord{}
}
