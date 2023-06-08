package api

import (
	"github.com/asim/proxy-gpt/ai"
	"github.com/asim/proxy-gpt/cache"
	"github.com/asim/proxy-gpt/db"
)

// saveContext is what we need to maintain our context cache
func saveContext(msg Message, context []ai.Context) {
	// we don't save the context of off-the-record data
	if msg.OTR {
		return
	}

	// append new prompt to
	context = append(context, ai.Context{
		Prompt: msg.Prompt,
		Reply:  msg.Reply,
	})

	// save the context
	cache.Set(msg.ChatID, context)
}

// get the context from cache
func getContext(chatID string) []ai.Context {
	// pull context from the cache
	var context []ai.Context
	cache.Get(chatID, &context)
	return context
}

// buildContext from the database
func buildContext(chatID string, limit int) ([]ai.Context, error) {
	var messages []Message

	// get messages
	res := db.Where("chat_id = ?", chatID).Order("created_at desc").Limit(limit).Find(&messages)
	if err := res.Error; err != nil {
		return nil, err
	}

	// reset the context
	context := []ai.Context{}

	// build new context
	for i := len(messages); i > 0; i-- {
		message := messages[i-1]

		// do not use off the record messages
		if message.OTR {
			continue
		}

		context = append(context, ai.Context{
			Prompt: message.Prompt,
			Reply:  message.Reply,
		})
	}

	return context, nil
}
