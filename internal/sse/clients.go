package sse

import (
	"sync"

	"github.com/debemdeboas/the-archive/internal/model"
)

type Client struct {
	Msg    chan string
	PostId model.PostId
}

type SSEClients struct {
	clients map[*Client]bool
	mu      sync.RWMutex
}

func NewSSEClients() *SSEClients {
	return &SSEClients{
		clients: make(map[*Client]bool),
	}
}

func (s *SSEClients) Add(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client] = true
}

func (s *SSEClients) Delete(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
	close(client.Msg)
}

func (s *SSEClients) Broadcast(postId model.PostId, msg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clients {
		if client.PostId == postId {
			select {
			case client.Msg <- msg:
			default:
			}
		}
	}
}
