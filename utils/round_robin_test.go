package utils

import (
	"testing"
)

func TestGetNextRPC_SingleRPC(t *testing.T) {
	rpcs := []string{"http://rpc1.example.com"}
	for i := 0; i < 10; i++ {
		got := GetNextRPC(rpcs)
		if got != rpcs[0] {
			t.Errorf("expected %s, got %s", rpcs[0], got)
		}
	}
}

func TestGetNextRPC_RoundRobin(t *testing.T) {
	rpcs := []string{"http://rpc1.example.com", "http://rpc2.example.com", "http://rpc3.example.com"}

	counts := make(map[string]int)
	n := 999 // divisible by 3 to get even distribution
	for i := 0; i < n; i++ {
		rpc := GetNextRPC(rpcs)
		counts[rpc]++
	}

	expected := n / len(rpcs)
	for _, rpc := range rpcs {
		if counts[rpc] != expected {
			t.Errorf("expected %d calls to %s, got %d", expected, rpc, counts[rpc])
		}
	}
}

func TestGetNextRPC_EmptyList(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty RPC list")
		}
	}()
	GetNextRPC([]string{})
}
