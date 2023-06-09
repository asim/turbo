package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/asim/proxy-gpt/ai"
	"github.com/asim/proxy-gpt/db"
	"github.com/asim/proxy-gpt/event"
	"github.com/asim/proxy-gpt/log"
	"github.com/google/uuid"
)

var (
	// Default number of prompt/replies to send to the llm
	DefaultContext = 10
)

// Chat is the base type for a conversation
type Chat struct {
	gorm.Model
	ID     string `json:"id" valid:"required" gorm:"index:idx_chat_user,priority:2"`
	Name   string `json:"name" valid:"required"` // name of the chat given by the user
	LLM    string `json:"model" valid:"required"`
	UserID string `json:"user_id" gorm:"index:idx_chat_user,priority:1"`
	GroupID string `json:"group_id" gorm:"index"` // TODO new composite index with user
}

// Message represents the messages in a Chat
type Message struct {
	gorm.Model
	ID     string `json:"id" valid:"required"`
	ChatID string `json:"chat_id" gorm:"index:idx_chat_message,priority:2"`
	UserID string `json:"user_id" gorm:"index:idx_chat_message,priority:1"`
	GroupID string `json:"group_id" gorm:"index"`
	Prompt string `json:"prompt"`
	Reply  string `json:"reply"`
	LLM    string `json:"model"`
	OTR    bool   `json:"otr"`
}

type ChatCreateRequest struct {
	Name   string `json:"name" valid:"required"`
	Model  string `json:"model" valid:"required"`
	GroupID string `json:"group_id"`
}

type ChatCreateResponse struct {
	Chat
}

type ChatUpdateRequest struct {
	ID    string `json:"id"`
	Name  string `json:"name" valid:"required"`
	Model string `json:"model" valid:"required"`
}

type ChatUpdateResponse struct {
	// Unique chat id
	ID    string `json:"id"`
	Name  string `json:"name" valid:"required"`
	Model string `json:"model" valid:"required"`
}

type ChatDeleteRequest struct {
	// Unique chat id
	ID string `json:"id" valid:"required"`
}

type ChatDeleteResponse struct {
	// no params
}

type ChatIndexRequest struct {
	// no params
}

type ChatIndexResponse struct {
	Chats []*Chat `json:"chats"`
}

type ChatReadRequest struct {
	ID string `json:"id" valid:"required"`
}

type ChatReadResponse struct {
	Chat     *Chat      `json:"chat"`
	Messages []*Message `json:"messages"`
	Users    []*User    `json:"users"`
}

type ChatPromptRequest struct {
	ID      string `json:"id" valid:"required"`
	Prompt  string `json:"prompt" valid:"required"`
	Context int    `json:"context,omitempty"`
	Stream  bool   `json:"stream,omitempty"`
	OTR     bool   `json:"otr,omitempty"`
}

type ChatPromptResponse struct {
	// If stream is specified in request then Reply in response message is nil
	Message Message `json:"message"`
}

type ChatUser struct {
	gorm.Model
	ChatID string `json:"chat_id" gorm:"uniqueIndex:idx_chat_user_member,priority:2"`
	UserID string `json:"user_id" gorm:"uniqueIndex:idx_chat_user_member,priority:1"`
}

type ChatUserAddRequest struct {
	ChatID string `json:"chat_id" valid:"required"`
	UserID string `json:"user_id" valid:"required"`
}

type ChatUserAddResponse struct{}

type ChatUserRemoveRequest struct {
	ChatID string `json:"chat_id"`
	UserID string `json:"user_id"`
}

type ChatUserRemoveResponse struct{}

type ChatStreamRequest struct {
	ID string `json:"id" valid:"required"`
}

type ChatStreamResponse struct {
	Message Message `json:"message"`
	Partial bool    `json:"partial"`
}

// CreateChat enables the creation of a new chat
func ChatCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	cc := new(ChatCreateRequest)
	cc.Name = r.Form.Get("name")
	cc.Model = r.Form.Get("model")
	cc.GroupID = r.Form.Get("group_id")

	if err := decode(r, cc); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// set default name
	if len(cc.Name) == 0 {
		cc.Name = "general"
	}

	// set model
	if len(cc.Model) == 0 {
		// use the default model
		cc.Model = ai.DefaultModel
	} else {
		// look up the model
		_, ok := ai.Models[cc.Model]
		if !ok {
			// TODO: return error its an unsupported model
			log.Printf("Unknown model for %v defaulting to %v\n", cc.Model, ai.DefaultModel)
			cc.Model = ai.DefaultModel
		}
	}

	var group *Group

	if len(cc.GroupID) > 0 {
		// lookup members
		var groupMember GroupMember
		err := db.Where("user_id = ? AND group_id = ?", sess.UserID, cc.GroupID).First(&groupMember).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// user is not a member of the group
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var og Group
		og.ID = cc.GroupID

		if err := db.First(&og).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// set group
		group = &og
	} else {
		var err error
		// get the user group
		group, err = GetGroup(sess.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// create a chat,
	chat := &Chat{
		ID:     uuid.New().String(),
		Name:   cc.Name,
		LLM:    cc.Model,
		GroupID: group.ID,
		UserID: sess.UserID,
	}

	// create the chat
	if res := db.Create(chat); res.Error != nil {
		log.Print("Error creating chat: ", res.Error)
		http.Error(w, "Error creating chat", http.StatusInternalServerError)
		return
	}

	// create a chat,
	chatUser := &ChatUser{
		ChatID: chat.ID,
		UserID: sess.UserID,
	}

	// create the chat user
	db.Create(chatUser)

	// TODO: call OpenAI to create a new chat
	// {"role": "system", "content": "You are a helpful assistant"}

	// respond to user
	respond(w, r, ChatCreateResponse{*chat})
}

// ChatDelete deletes a chat
func ChatDelete(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c := new(ChatDeleteRequest)
	c.ID = r.Form.Get("id")

	if err := decode(r, c); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// check the owner
	var chat Chat
	chat.ID = c.ID
	if res := db.First(&chat); res.Error != nil {
		http.Error(w, res.Error.Error(), http.StatusInternalServerError)
		return
	}

	// unauthorized
	if sess.UserID != chat.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// delete chat
	// TODO: validate the data so we don't delete all chats
	res := db.Where("user_id = ?", sess.UserID).Delete(&Chat{ID: c.ID})
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// delete the chat users
	db.Where("chat_id = ?", chat.ID).Delete(&ChatUser{})

	// respond to user
	respond(w, r, ChatDeleteResponse{})
}

// ChatIndex returns all chats for a user
func ChatIndex(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// load additional chats for a user
	chatUsers, err := GetChatsForUser(sess.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// make list of chat ids
	var ids []string
	for _, id := range chatUsers {
		ids = append(ids, id.ChatID)
	}

	var chats []Chat

	// list chats
	res := db.Order("updated_at desc").Where("user_id = ?", sess.UserID).Or("id IN ?", ids).Find(&chats)

	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := &ChatIndexResponse{}
	idMap := map[string]bool{}

	// append list of chats
	for _, ch := range chats {
		// already seen
		if _, ok := idMap[ch.ID]; ok {
			continue
		}

		// set as seen id
		idMap[ch.ID] = true

		// append chat
		chat := ch
		resp.Chats = append(resp.Chats, &chat)
	}

	// respond to user
	respond(w, r, resp)
}

func ChatUpdate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c := new(ChatUpdateRequest)
	c.ID = r.Form.Get("id")
	c.Name = r.Form.Get("name")

	if err := decode(r, c); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var chat Chat
	// set id
	chat.ID = c.ID

	if err := db.First(&chat).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if sess.UserID != chat.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// set name
	chat.Name = c.Name

	res := db.Update(chat)
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// respond to user
	respond(w, r, ChatUpdateResponse{})
}

// ChatRead returns the messages for a chat
func ChatRead(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c := new(ChatReadRequest)
	c.ID = r.Form.Get("chat_id")

	if err := decode(r, c); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var chat Chat
	var messages []Message

	// get the chat
	res := db.Where("id = ?", c.ID).First(&chat)
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// get chat users
	chatUsers, err := GetChatUsers(chat.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seen := map[string]bool{}

	// check if the user is in that chat group
	for _, user := range chatUsers {
		if user.UserID == sess.UserID {
			seen[user.UserID] = true
			break
		}
	}

	// unauthorized to view chat, not in group
	if !seen[sess.UserID] && chat.UserID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// get list fo user ids
	var ids []string
	deleted := map[string]db.DeletedAt{}

	for _, user := range chatUsers {
		if user.DeletedAt.Valid {
			deleted[user.UserID] = db.DeletedAt(user.DeletedAt)
		}

		ids = append(ids, user.UserID)
	}

	// check we're seeing ourselves
	seen = map[string]bool{}

	for _, id := range ids {
		if id == sess.UserID {
			seen[id] = true
			break
		}
	}

	// our own user is not listed in chat users
	if !seen[sess.UserID] {
		ids = append(ids, sess.UserID)
	}

	// check the owner id
	if chat.UserID != sess.UserID && !seen[chat.UserID] {
		ids = append(ids, chat.UserID)
	}

	// get the users
	users, err := GetUsers(ids)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// get messages
	res = db.Where("chat_id = ?", chat.ID).Order("created_at").Find(&messages)
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// create chat history
	resp := &ChatReadResponse{
		Chat: &chat,
	}

	// append messages
	for _, m := range messages {
		msg := m
		resp.Messages = append(resp.Messages, &msg)
	}

	// append users
	for _, u := range users {
		usr := u

		// ultra ugly hack to get chat user deleted time
		// TODO encapsulate the user with status/role, etc
		if t, ok := deleted[usr.ID]; ok {
			usr.DeletedAt = t
		}

		resp.Users = append(resp.Users, &usr)
	}

	// respond to user
	respond(w, r, resp)
}

// ChatPrompt is for making a request to the ChatGPT platform
func ChatPrompt(w http.ResponseWriter, r *http.Request) {
	var chat Chat
	var sess Session

	r.ParseForm()

	// attempt to pull user session from context
	if s, ok := r.Context().Value(Session{}).(*Session); !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	} else {
		sess = *s
	}

	// parse the request
	c := new(ChatPromptRequest)
	c.ID = r.Form.Get("id")
	c.Prompt = r.Form.Get("prompt")

	if v := r.Form.Get("context"); len(v) > 0 {
		c.Context, _ = strconv.Atoi(v)
	} else {
		// set a default
		c.Context = DefaultContext
	}

	// stream back response via /chat/stream
	if v := r.Form.Get("stream"); v == "true" {
		c.Stream = true
	}

	// get otr flag
	if v := r.Form.Get("otr"); v == "true" {
		c.OTR = true
	}

	// set the default context limit
	if c.Context > DefaultContext || c.Context < 0 {
		c.Context = DefaultContext
	}

	if err := decode(r, c); err != nil {
		http.Error(w, "Invalid request", http.StatusInternalServerError)
		return
	}

	chatID := c.ID
	prompt := c.Prompt

	// validate the chat
	chat.ID = chatID

	// get the chat
	res := db.Where("id = ?", chatID).First(&chat)
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// get chat users
	chatUsers, err := GetChatUsers(chat.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var seen bool

	// check if the user is in that chat group
	for _, user := range chatUsers {
		if user.UserID == sess.UserID {
			seen = true
			break
		}
	}

	// unauthorized to view chat, not in group
	if !seen && chat.UserID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// make request on behalf of user
	// TODO: decide whether we're going to internally proxy to /v1/
	// or if we're going to just do the openai magic right here
	key := fmt.Sprintf("%s-%s", sess.UserID, chatID)

	// TODO: decide how context applies to a chat
	user := base64.StdEncoding.EncodeToString([]byte(key))

	// define the message
	m := &Message{
		ID:     uuid.New().String(),
		Prompt: prompt,
		ChatID: chatID,
		UserID: sess.UserID,
		GroupID: chat.GroupID,
		LLM:    chat.LLM,
		OTR:    c.OTR,
	}

	// with the stream we have to wait
	wait := make(chan *Message, 1)

	// pull context from the cache
	context := getContext(chat.ID)

	// send it to the LLM if it's not off the record
	if !c.OTR {
		// if there's not enough context attempt to get it from the chat
		if len(context) < c.Context {
			var err error
			context, err = buildContext(chat.ID, c.Context)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// cap number of messages we send
		for len(context) > c.Context {
			context = context[1:]
		}

		// get the model
		model, ok := ai.Models[chat.LLM]
		if !ok {
			log.Printf("Unsupported model %v defaulting to %v\n", chat.LLM, ai.DefaultModel)
			model = ai.Models[ai.DefaultModel]
		}

		// if asked for a streaming response we run this in a go routine
		if c.Stream {
			words, err := model.Stream(prompt, user, context...)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// stream the words, we can choose to do this async too
			go streamWords(r, &sess, chat, words, wait, context)
		} else {
			// non streaming response, complete the prompt and reply inline
			reply, err := model.Complete(prompt, user, context...)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// set reply
			m.Reply += reply
		}
	}

	// update the chat to indicate it's been updated
	chat.UpdatedAt = time.Now()

	go db.Update(&chat)

	// write response to database
	if res := db.Create(m); res.Error != nil {
		log.Print("Error saving message", res.Error)
		http.Error(w, "Error saving message", http.StatusInternalServerError)
		return
	}

	ch := &ChatStreamResponse{
		Message: *m,
		Partial: c.Stream, // true if streaming
	}

	// save the context now if we're not streaming
	if !c.Stream {
		// set as non partial response
		ch.Partial = false
		// save context immediately
		saveContext(*m, context)
	}

	// written the db record, keep going
	wait <- m
	close(wait)

	// publish event
	event.Publish(chatID, ch)

	// write response to user
	respond(w, r, ChatPromptResponse{
		Message: *m,
	})
}

func ChatUserAdd(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	cc := new(ChatUserAddRequest)
	cc.ChatID = r.Form.Get("chat_id")
	cc.UserID = r.Form.Get("user_id")

	if err := decode(r, cc); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// get the chat
	chat, err := GetChat(cc.ChatID)
	if err != nil {
		http.Error(w, "Failed to get chat", http.StatusInternalServerError)
		return
	}

	if chat.UserID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// create a chat,
	chatUser := &ChatUser{
		ChatID: cc.ChatID,
		UserID: cc.UserID,
	}

	// create the chat
	res := db.Create(chatUser)
	if res.Error != nil && strings.Contains(res.Error.Error(), "duplicate key") {
		// try again
		log.Print("User exists, trying to undelete")
		res = db.Unscoped().Model(&ChatUser{}).Where(
			"chat_id = ? AND user_id = ?",
			chatUser.ChatID, chatUser.UserID,
		).Update("deleted_at", nil)
	}

	// we have tried everything to create the user
	if res.Error != nil {
		log.Print("Error creating chat user: ", res.Error)
		http.Error(w, "Error creating chat user", http.StatusInternalServerError)
		return
	}

	// respond to user
	respond(w, r, ChatUserAddResponse{})
}

func ChatUserRemove(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c := new(ChatUserRemoveRequest)
	c.ChatID = r.Form.Get("chat_id")
	c.UserID = r.Form.Get("user_id")

	if err := decode(r, c); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// get the chat
	chat, err := GetChat(c.ChatID)
	if err != nil {
		http.Error(w, "Failed to get chat", http.StatusInternalServerError)
		return
	}

	if chat.UserID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// delete chat
	// TODO: validate the data so we don't delete all chats
	res := db.Where("user_id = ? AND chat_id = ?", c.UserID, c.ChatID).Delete(&ChatUser{})
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// respond to user
	respond(w, r, ChatUserRemoveResponse{})
}

// ChatStream is for streaming SSE events from a chat
func ChatStream(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c := new(ChatStreamRequest)
	c.ID = r.Form.Get("id")

	if err := decode(r, c); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var chat Chat

	// get the chat
	res := db.Where("id = ?", c.ID).First(&chat)
	if err := res.Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// get chat users
	chatUsers, err := GetChatUsers(chat.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seen := map[string]bool{}

	// check if the user is in that chat group
	for _, user := range chatUsers {
		if user.UserID == sess.UserID {
			seen[user.UserID] = true
			break
		}
	}

	// unauthorized to view chat, not in group
	if !seen[sess.UserID] && chat.UserID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// headers required for SSE event stream
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	sub, err := event.Subscribe(chat.ID)
	if err != nil {
		log.Print("Failed event subscription", err)
		http.Error(w, "Error subscribing to stream", http.StatusInternalServerError)
		return
	}
	defer event.Unsubscribe(sub)

	// serve a socket
	if isWebSocket(r) {
		serveWebSocket(w, r, sub)
		return
	}

	// otherwise do SSE
	w.Header().Set("Content-Type", "text/event-stream")

	for {
		var msg json.RawMessage
		if err := sub.Next(r.Context(), &msg); err != nil {
			http.Error(w, err.Error(), 499)
			return
		}
		fmt.Fprintf(w, "data: %v\n\n", string(msg))

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func GetChat(id string) (*Chat, error) {
	var chat Chat
	chat.ID = id

	// get the chat
	res := db.First(&chat)
	if err := res.Error; err != nil {
		return nil, err
	}

	return &chat, nil
}

func GetChatUser(chatID, userID string) (*ChatUser, error) {
	var user ChatUser
	res := db.Where("chat_id = ? AND user_id = ?", chatID, userID).First(&user)
	if res.Error != nil {
		return nil, res.Error
	}
	return &user, nil
}

func GetChatUsers(id string) ([]ChatUser, error) {
	var users []ChatUser

	// get chat users (including delete)
	res := db.Unscoped().Where("chat_id = ?", id).Find(&users)
	if err := res.Error; err != nil {
		return nil, err
	}

	return users, nil
}

func GetChatsForUser(id string) ([]ChatUser, error) {
	var users []ChatUser

	// get messages
	res := db.Where("user_id = ?", id).Find(&users)
	if err := res.Error; err != nil {
		return nil, err
	}

	return users, nil
}

func streamWords(r *http.Request, sess *Session, chat Chat, words chan string, wait chan *Message, context []ai.Context) {
	var reply string

	// make message copy
	var msg Message

	// wait till we're past the gate
	m := <-wait

	// set id
	msg.ID = m.ID

	// update the record
	res := db.First(&msg)
	if res.Error != nil {
		// TODO: response with error
		log.Print("Error getting message", res.Error)
		return
	}

	for {
		select {
		case word, ok := <-words:
			if !ok {
				// we're done, save context and leave
				saveContext(msg, context)

				// set the reply
				msg.Reply = reply

				ch := &ChatStreamResponse{
					Message: msg,
					Partial: false,
				}

				// publish the whole thing
				event.Publish(msg.ChatID, ch)

				// update record
				db.Update(msg)

				// done
				return
			}

			// add to the reply
			reply += word
			// set the word
			msg.Reply = word

			// publish the message
			event.Publish(msg.ChatID, &ChatStreamResponse{
				Message: msg,
				Partial: true,
			})
		}
	}
}
