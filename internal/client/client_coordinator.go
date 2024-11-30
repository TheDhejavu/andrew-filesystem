package client

import "sync"

type Coordinator struct {
	mu sync.Mutex
}

func NewCoordinator() *Coordinator {
	return &Coordinator{}
}
