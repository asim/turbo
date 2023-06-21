package main

import (
	"os"

	"github.com/asim/turbo/ai"
	"github.com/asim/turbo/api"
	"github.com/asim/turbo/cache"
	"github.com/asim/turbo/db"
	"github.com/asim/turbo/event"
	"github.com/asim/turbo/log"
)

var (
	// OpenAI address e.g azure openai service
	Url = os.Getenv("OPENAI_API_URL")
	// key for the OpenAI API
	Key = os.Getenv("OPENAI_API_KEY")
	// Address of the http server
	Address = os.Getenv("ADDRESS")
	// Infrastructure settings
	Redis = os.Getenv("REDIS_ADDRESS")
	DB    = os.Getenv("DB_ADDRESS")
)

func main() {
	// proxy api configuration
	if len(Key) == 0 {
		log.Print("missing OPENAI_API_KEY")
		os.Exit(1)
	}

	// set the default api url
	if len(Url) == 0 {
		Url = ai.DefaultURL
	}

	if err := db.Init(DB); err != nil {
		log.Print("failed to connect to db:", err)
		os.Exit(1)
	}

	// TODO: register types in db
	db.Migrate(
		// chats
		&api.Chat{},
		// chat users
		&api.ChatUser{},
		// proxy events
		&api.Event{},
		// chat messages
		&api.Message{},
		// user accounts
		&api.User{},
		// user sessions
		&api.Session{},
		// groups
		&api.Group{},
		// group members
		&api.GroupMember{},
	)

	// setup the cache
	cache.Init(Redis)
	// setup events
	event.Init(Redis)

	// new proxy
	p := api.New(&api.Options{
		Key: Key,
		Url: Url,
	})

	// register api routes
	p.Register(api.Routes)

	// setup openai
	if err := ai.Set(Key, Url); err != nil {
		log.Print("Failed to setup AI", err)
		os.Exit(1)
	}

	// add middleware

	// with event logger
	hw := api.WithLogger(p)
	// with auth / admin
	hw = api.WithAuth(hw)
	// with cors
	hw = api.WithCors(hw)

	// Set address if not specified
	if len(Address) == 0 {
		Address = ":8080"
	}

	// run the api
	if err := p.Run(Address, hw); err != nil {
		log.Fatal(err)
	}
}
