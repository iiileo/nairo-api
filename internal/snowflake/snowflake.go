package snowflake

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	mu     sync.Mutex
	node   *snowflake.Node
	nodeID int64 = 1
)

// Configure sets worker node id (0-1023). Call before first NextID.
func Configure(id int64) error {
	mu.Lock()
	defer mu.Unlock()
	n, err := snowflake.NewNode(id)
	if err != nil {
		return fmt.Errorf("snowflake node: %w", err)
	}
	node = n
	nodeID = id
	return nil
}

func NextID() (int64, error) {
	mu.Lock()
	defer mu.Unlock()
	if node == nil {
		n, err := snowflake.NewNode(nodeID)
		if err != nil {
			return 0, fmt.Errorf("snowflake init: %w", err)
		}
		node = n
	}
	return node.Generate().Int64(), nil
}

func MustID() int64 {
	id, err := NextID()
	if err != nil {
		panic(err)
	}
	return id
}
