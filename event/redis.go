package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type redisBroker struct {
	Client redis.UniversalClient
}

func newRedisBroker(addr string) (*redisBroker, error) {
	if len(addr) == 0 {
		addr = "redis://127.0.0.1:6379"
	}
	redisOptions, err := redis.ParseURL(addr)
	if err != nil {
		return nil, err
	}
	return &redisBroker{redis.NewClient(redisOptions)}, nil
}

func (r *redisBroker) Publish(topic string, ev interface{}) error {
	b, err := json.Marshal(ev)
	if err != nil {
		fmt.Println("ERROR")
		return err
	}

	fmt.Printf("[redis] publishing %+v\n", string(b))
	if err := r.Client.Publish(context.Background(), topic, string(b)).Err(); err != nil {
		return err
	}

	return nil
}

func (r *redisBroker) Subscribe(topic string) (*Subscriber, error) {
	ctx := context.Background()
	sub := r.Client.Subscribe(ctx, topic)

	// Create a new subscriber
	s := &Subscriber{
		ID:    uuid.New().String(),
		Topic: topic,
		Chan:  make(chan []byte, 100),
		Exit:  make(chan bool),
	}

	// Start a goroutine to read messages from Redis
	go func() {
		defer close(s.Chan)

		for {
			fmt.Println("[redis] waiting on message")
			select {
			case msg := <-sub.Channel():
				fmt.Printf("[redis] sub message %+v\n", msg.Payload)
				s.Lock()
				s.Queue = append(s.Queue, []byte(msg.Payload))
				s.Unlock()
				select {
				case s.Chan <- []byte(msg.Payload):
				case <-s.Exit:
					return
				}
			case <-s.Exit:
				sub.Unsubscribe(context.Background(), s.Topic)
				return
			}
		}
	}()

	return s, nil
}

func (r *redisBroker) Unsubscribe(s *Subscriber) error {
	if err := s.Close(); err != nil {
		return err
	}
	return nil
}
