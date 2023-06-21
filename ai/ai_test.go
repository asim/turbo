package ai

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComplete(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY env var not set")
	}

	err := Set(apiKey, DefaultURL)
	assert.NoError(t, err)

	resp, err := Complete("Hello, ChatGPT", "User", Context{
		Prompt: "This is a test",
	})
	t.Log("complete response", resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
}

func TestStream(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY env var not set")
	}

	err := Set(apiKey, DefaultURL)
	assert.NoError(t, err)

	ch, err := Stream("Hello ChatGPT", "User", Context{
		Prompt: "This is a test",
	})
	assert.NoError(t, err)

	for {
		message, ok := <-ch
		t.Log("stream response", message)
		if !ok {
			break
		}
	}
}
