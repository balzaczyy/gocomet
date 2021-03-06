package gocomet

import (
	"fmt"
	"log"
	"sync"
)

type Message struct {
	channel string
	data    string
}

func (msg *Message) String() string {
	return fmt.Sprintf("@%v: %v", msg.channel, msg.data)
}

/*
A simple Message Broker that transmits text messages between clients
through subscribed channels.
*/
type Broker struct {
	*sync.RWMutex
	clients map[string]chan *Message
	router  *Router
	rules   map[string]map[string]*Rule
}

/*
Creates a message broker instance.
*/
func newBroker() *Broker {
	return &Broker{
		RWMutex: &sync.RWMutex{},
		clients: make(map[string]chan *Message),
		router:  newRouter(),
		rules:   make(map[string]map[string]*Rule),
	}
}

/*
Register a new client and obtain its designated channel.
*/
func (b *Broker) register(clientId string) chan *Message {
	b.Lock()
	defer b.Unlock()

	ch, ok := b.clients[clientId]
	if !ok {
		ch = make(chan *Message)
		b.clients[clientId] = ch
		b.rules[clientId] = make(map[string]*Rule)
	}
	return ch
}

/*
Deregister an existing client and release all its subscribed channels.
*/
func (b *Broker) deregister(clientId string) {
	b.Lock()
	defer b.Unlock()
	if ch, ok := b.clients[clientId]; ok {
		delete(b.clients, clientId)
		close(ch) // close the channel
	}
	delete(b.rules, clientId)
}

/*
Subscribe the client to the channel. After that, the client's own
channel can get messages when others broadcast messages to the
subscribed channel.
*/
func (b *Broker) subscribe(clientId, channel string) {
	if !b.hasClient(clientId) {
		return // client ID not exists
	}

	rule := b.router.add(channel, clientId)

	b.Lock()
	defer b.Unlock()

	b.rules[clientId][channel] = rule
}

func (b *Broker) hasClient(clientId string) (ok bool) {
	b.RLock()
	defer b.RUnlock()
	_, ok = b.clients[clientId]
	return
}

/*
Unsubscribe the client from the channel. After that, the future
messages or pending messages are ceased.
*/
func (b *Broker) unsubscribe(clientId, channel string) bool {
	if !b.hasClient(clientId) {
		return false // client ID not exists
	}

	b.Lock()
	defer b.Unlock()

	if rule, ok := b.rules[clientId][channel]; ok {
		rule.remove()
		delete(b.rules[clientId], channel)
		return true
	}
	return false
}

/*
Broadcast the message to the given channel. This method is supposed
to be non-blocking style iff the target channels are actively
monitored. The broker client may choose to implement a different
strategy, like message ordering or persistence. The broker doesn't
guarrantee message delivery though.
*/
func (b *Broker) broadcast(channel, msg string) {
	targets := b.router.run(channel)
	if len(targets) > 0 {
		log.Printf("[Broker]Broadcast to %v", targets)
		for _, c := range targets {
			b.send(c, &Message{channel, msg})
		}
	}
}

func (b *Broker) send(client string, msg *Message) {
	b.RLock()
	ch := b.clients[client]
	b.RUnlock()
	log.Printf("[%8.8v]Receiving message: %v", client, msg)
	ch <- msg
}
