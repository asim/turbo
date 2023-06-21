package ai

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/asim/turbo/log"
	"github.com/sashabaranov/go-openai"
)

var (
	Client *openai.Client

	// DefaultModel
	DefaultModel = "gpt-3"

	DefaultURL = "https://api.openai.com/v1"

	// limit max tokens
	DefaultLimit = 4096
)

var (
	// Supported models platform/model
	Models = map[string]Model{
		"gpt-4": &chatgpt{openai.GPT4},
		"gpt-3": &chatgpt{openai.GPT3Dot5Turbo},
	}
)

// Model represents a model which can be sent a prompt
type Model interface {
	Complete(prompt, user string, ctx ...Context) (string, error)
	Stream(prompt, user string, ctx ...Context) (chan string, error)
	String() string
}

// Context represents past prompts to a model
type Context struct {
	Prompt string
	Reply  string
}

// Set the api key for a given url
func Set(key, uri string) error {
	if strings.Contains(uri, "openai.azure.com") {
		// setup config
		cfg := openai.DefaultAzureConfig(key, uri)
		Client = openai.NewClientWithConfig(cfg)
		return nil
	} else if len(uri) > 0 {
		// setup config
		cfg := openai.DefaultConfig(key)
		// set base uri
		cfg.BaseURL = uri
		// set client
		Client = openai.NewClientWithConfig(cfg)
		return nil
	}

	// default url
	Client = openai.NewClient(key)
	return nil
}

// chatgpt models
type chatgpt struct {
	model string
}

func (c *chatgpt) Complete(prompt, user string, ctx ...Context) (string, error) {
	// create chat completion
	resp, err := Client.CreateChatCompletion(
		context.Background(),
		complete(prompt, user, c.model, ctx...),
	)
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *chatgpt) Stream(prompt, user string, ctx ...Context) (chan string, error) {
	req := complete(prompt, user, c.model, ctx...)
	req.Stream = true

	stream, err := Client.CreateChatCompletionStream(context.TODO(), req)
	if err != nil {
		log.Printf("Error creating chat stream: %v\n", err)
		return nil, err
	}

	ch := make(chan string, 100)

	go func() {
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				log.Printf("EOF in ai chat stream: %v\n", err)
				close(ch)
				return
			}

			if err != nil {
				log.Printf("Error in ai chat stream: %v\n", err)
				close(ch)
				return
			}

			ch <- response.Choices[0].Delta.Content
		}

	}()

	return ch, nil
}

func (c *chatgpt) String() string {
	return c.model
}

func complete(prompt, user, model string, ctx ...Context) openai.ChatCompletionRequest {
	// we need to set a context limit
	limit := DefaultLimit - len(prompt)

	message := []openai.ChatCompletionMessage{}

	for _, c := range ctx {
		// adjust the limit
		limit -= (len(c.Prompt) + len(c.Reply))

		// we're hitting the limit
		if limit < 0 {
			break
		}

		// set the user message
		message = append(message, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: c.Prompt,
		})
		// set the assistant response
		message = append(message, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: c.Reply,
		})
	}

	// append the actual next prompt
	message = append(message, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})

	return openai.ChatCompletionRequest{
		Model:    model,
		Messages: message,
		User:     user,
	}
}

// Complete a request
func Complete(prompt, user string, ctx ...Context) (string, error) {
	md, ok := Models[DefaultModel]
	if !ok {
		return "", errors.New("Unsupported model")
	}
	resp, err := md.Complete(prompt, user, ctx...)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// Stream a request
func Stream(prompt, user string, ctx ...Context) (chan string, error) {
	md, ok := Models[DefaultModel]
	if !ok {
		return nil, errors.New("Unsupported model")
	}
	ch, err := md.Stream(prompt, user, ctx...)
	if err != nil {
		log.Printf("Error creating chat stream: %v\n", err)
		return nil, err
	}
	return ch, nil
}
