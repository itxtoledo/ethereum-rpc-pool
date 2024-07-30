package utils

import (
	"sync/atomic"
)

var currentIndex uint32

// Function to get the next RPC using round robin
func GetNextRPC(rpcs []string) string {
	index := atomic.AddUint32(&currentIndex, 1)
	return rpcs[(index-1)%uint32(len(rpcs))]
}
