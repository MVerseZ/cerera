package sync

import "testing"

func TestShouldSyncChain(t *testing.T) {
	tests := []struct {
		name        string
		peerHeight  int
		localHeight int
		want        bool
	}{
		{"peer one ahead", 6, 5, true},
		{"peer equal height", 5, 5, false},
		{"peer behind", 4, 5, false},
		{"from genesis", 1, 0, true},
		{"one block lag old bug", 6, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSyncChain(tt.peerHeight, tt.localHeight); got != tt.want {
				t.Fatalf("shouldSyncChain(%d, %d) = %v, want %v", tt.peerHeight, tt.localHeight, got, tt.want)
			}
		})
	}
}
