package event

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type ExampleEvent struct {
	Message string `json:"message"`
}

func TestMemoryBroker(t *testing.T) {
	// Subscribe to a topic
	sub, err := Subscribe("test_topic")
	assert.NoError(t, err)

	// Publish an event to the topic
	ev := ExampleEvent{Message: "Hello World!"}
	err = Publish("test_topic", ev)
	assert.NoError(t, err)

	// Wait for the event to be delivered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var received ExampleEvent
	err = sub.Next(ctx, &received)
	assert.NoError(t, err)
	assert.Equal(t, ev.Message, received.Message)

	// Unsubscribe from the topic
	err = Unsubscribe(sub)
	assert.NoError(t, err)
}

func TestMemoryBroker_Subscriber_Queue(t *testing.T) {
	// Subscribe to a topic
	sub, err := Subscribe("test_topic")
	assert.NoError(t, err)

	// Publish two events to the topic
	ev1 := ExampleEvent{Message: "Event 1"}
	err = Publish("test_topic", ev1)
	assert.NoError(t, err)

	ev2 := ExampleEvent{Message: "Event 2"}
	err = Publish("test_topic", ev2)
	assert.NoError(t, err)

	// Wait for the events to be delivered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var received1 ExampleEvent
	err = sub.Next(ctx, &received1)
	assert.NoError(t, err)
	assert.Equal(t, ev1.Message, received1.Message)

	var received2 ExampleEvent
	err = sub.Next(ctx, &received2)
	assert.NoError(t, err)
	assert.Equal(t, ev2.Message, received2.Message)

	// Ensure the subscriber's queue is empty
	var received3 ExampleEvent
	err = sub.Next(ctx, &received3)
	assert.True(t, errors.Is(err, io.EOF))

	// Unsubscribe from the topic
	err = Unsubscribe(sub)
	assert.NoError(t, err)
}

func TestMemoryBroker_Subscriber_Close(t *testing.T) {
	// Subscribe to a topic
	sub, err := Subscribe("test_topic")
	assert.NoError(t, err)

	// Close the subscriber's exit channel
	err = sub.Close()
	assert.NoError(t, err)

	// Publish an event to the topic
	ev := ExampleEvent{Message: "Hello World!"}
	err = Publish("test_topic", ev)
	assert.NoError(t, err)

	// The event should not be received
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var received ExampleEvent
	err = sub.Next(ctx, &received)
	assert.True(t, errors.Is(err, io.EOF))

	// Unsubscribe from the topic
	err = Unsubscribe(sub)
	assert.NoError(t, err)
}

func TestInit(t *testing.T) {
	// Test the memory broker
	Init("memory")
	_, ok := Broker.(*memBroker)
	assert.True(t, ok)

	// TODO: Test the redis broker (not implemented)
}
