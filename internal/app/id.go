package app

import (
	"fmt"
	"time"
)

func NewJobID() string {
	return fmt.Sprintf("job-%d", time.Now().UTC().UnixNano())
}
