package api

import (
	"os"
	"testing"

	"github.com/asim/proxy-gpt/ai"
	"github.com/asim/proxy-gpt/cache"
	"github.com/asim/proxy-gpt/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func cleanup() {
	os.Remove("proxy.db")
}

func TestSaveAndGetContext(t *testing.T) {
	defer func() {
		cleanup()
	}()

	// Initialize the cache
	cache.Init("")

	// Initialize the database
	db.Init("")

	// migration
	db.Migrate(&Message{})

	// Add a message to the database
	message := Message{
		ChatID: "123",
		Prompt: "Hello",
		Reply:  "Hi there!",
	}
	db.Create(&message)

	// Save the context for the message
	saveContext(message, []ai.Context{})

	// Get the context for the message
	context := getContext(message.ChatID)

	// Assert that the context is not empty
	assert.NotEmpty(t, context)

	// Assert that the context contains the message
	assert.Equal(t, context[0].Prompt, message.Prompt)
	assert.Equal(t, context[0].Reply, message.Reply)
}

func TestBuildContext(t *testing.T) {
	defer func() {
		cleanup()
	}()

	// Initialize the database
	db.Init("")

	// migration
	db.Migrate(&Message{})

	// Add some messages to the database
	messages := []Message{
		{
			ID:     uuid.New().String(),
			ChatID: "123",
			Prompt: "Hello",
			Reply:  "Hi there!",
			LLM:    "gpt-4",
		},
		{
			ID:     uuid.New().String(),
			ChatID: "123",
			Prompt: "How are you?",
			Reply:  "I'm good, thanks!",
			LLM:    "gpt-4",
		},
		{
			ID:     uuid.New().String(),
			ChatID: "123",
			Prompt: "What are you up to?",
			Reply:  "Not much, just chilling",
			LLM:    "gpt-4",
			OTR:    true,
		},
	}
	for _, message := range messages {
		db.Create(&message)
	}

	// Build the context
	context, err := buildContext("123", 5)

	// Assert that there was no error building the context
	assert.NoError(t, err)

	// Assert that the context is the expected length
	assert.Len(t, context, 2)

	// Assert that the context does not contain the off-the-record message
	for _, c := range context {
		assert.NotEqual(t, c.Prompt, messages[2].Prompt)
	}
}
