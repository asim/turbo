package event

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// TODO: implement distrbuted broker
	Broker broker = newMemoryBroker()
)

type memBroker struct {
	sync.RWMutex
	subs map[string][]*Subscriber
}

type broker interface {
	Publish(string, interface{}) error
	Subscribe(string) (*Subscriber, error)
	Unsubscribe(*Subscriber) error
}

type Subscriber struct {
	ID    string
	Topic string
	Chan  chan []byte
	Exit  chan bool

	sync.RWMutex
	Queue [][]byte
}

type Request struct {
	Topic string
	Reply string
	Body  []byte
}

type Response struct {
	Body  []byte
	Error string
}

func newMemoryBroker() *memBroker {
	return &memBroker{
		subs: make(map[string][]*Subscriber),
	}
}

func (m *memBroker) Publish(topic string, ev interface{}) error {
	m.Lock()

	subs, ok := m.subs[topic]
	if !ok {
		m.Unlock()
		return nil
	}
	m.Unlock()

	b, _ := json.Marshal(ev)

	for i, sub := range subs {
		sub.Lock()
		sub.Queue = append(sub.Queue, b)
		m.subs[topic][i] = sub
		sub.Unlock()

		go func(sub *Subscriber, msg []byte) {
			// send message
			select {
			case sub.Chan <- b:
			case <-sub.Exit:
			}
		}(sub, b)
	}

	return nil
}

func (m *memBroker) Subscribe(topic string) (*Subscriber, error) {
	sub := make(chan []byte, 100)

	m.Lock()
	defer m.Unlock()

	s := &Subscriber{
		ID:    uuid.New().String(),
		Topic: topic,
		Chan:  sub,
		Exit:  make(chan bool),
	}

	// append subscriber
	m.subs[topic] = append(m.subs[topic], s)

	return s, nil
}

func (s *Subscriber) Next(ctx context.Context, ev interface{}) error {
	select {
	case _, ok := <-s.Chan:
		if !ok {
			return errors.New("subscription closed")
		}
		// pull from the queu
		s.Lock()
		if len(s.Queue) == 0 {
			return nil
		}
		msg := s.Queue[0]
		s.Queue = s.Queue[1:]
		s.Unlock()

		if err := json.Unmarshal(msg, ev); err != nil {
			return err
		}
		return nil
	case <-s.Exit:
		return io.EOF
	case <-ctx.Done():
		return io.EOF
	}
}

func (m *memBroker) Unsubscribe(s *Subscriber) error {
	m.Lock()
	defer m.Unlock()

	var subs []*Subscriber

	v, ok := m.subs[s.Topic]
	if !ok {
		return nil
	}

	for _, sub := range v {
		if sub.ID == s.ID {
			continue
		}
		subs = append(subs, sub)
	}

	m.subs[s.Topic] = subs
	return nil
}

func (s *Subscriber) Close() error {
	select {
	case <-s.Exit:
		return nil
	default:
		close(s.Exit)
		return nil
	}
}

func Publish(topic string, ev interface{}) error {
	return Broker.Publish(topic, ev)
}

func Subscribe(topic string) (*Subscriber, error) {
	return Broker.Subscribe(topic)
}

func Unsubscribe(s *Subscriber) error {
	return Broker.Unsubscribe(s)
}

func Init(addr string) error {
	if strings.HasPrefix(addr, "redis") {
		if c, err := newRedisBroker(addr); err != nil {
			return err
		} else {
			Broker = c
		}
		return nil
	}
	Broker = newMemoryBroker()
	return nil
}

func Query(topic string, request, response interface{}) error {
	id := uuid.New().String()
	sub, err := Subscribe(id)
	if err != nil {
		return err
	}
	defer Unsubscribe(sub)

	b, _ := json.Marshal(request)

	req := &Request{
		Topic: topic,
		Reply: id,
		Body:  b,
	}

	if err := Publish(topic, req); err != nil {
		return err
	}

	var rsp Response

	ctx, cancel := context.WithDeadline(context.TODO(), time.Now().Add(10*time.Second))
	defer cancel()

	if err := sub.Next(ctx, &rsp); err != nil {
		return err
	}

	if len(rsp.Error) > 0 {
		return errors.New(rsp.Error)
	}

	return json.Unmarshal(rsp.Body, response)
}
