package turbo

import (
	"net/http"
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

// App is the turbo app
type App struct {
	// the internal http proxy
	Proxy *api.Proxy
	// http proxy wrapper
	Handler http.Handler
}

// A http endpoint
type Endpoint struct {
	// The http handler
	Handler http.HandlerFunc
	// Whether it's authenticated
	Private bool
}

// Perform a database migration
func (a *App) Migrate(vals ...interface{}) error {
	// migrate the user vals
	return db.Migrate(vals...)
}

// Register api routes as endpoint/handler e.g /foobar is the key
func (a *App) Register(path string, ep Endpoint) {
	a.Proxy.Register(map[string]http.HandlerFunc{
		path: ep.Handler,
	})
	if !ep.Private {
		// add to the excludes
		api.Excludes = append(api.Excludes, path)
	}
}

// Run the app on the given address e.g Run(":8080")
func (a *App) Run() {
	// Set address if not specified
	if len(Address) == 0 {
		Address = ":8080"
	}

	// run the api
	log.Print("Running on", Address)

	if err := a.Proxy.Run(Address, a.Handler); err != nil {
		log.Fatal(err)
	}
}

// Create a new turbo app
func New() *App {
	// set the default api url
	if len(Url) == 0 {
		Url = ai.DefaultURL
	}

	if err := db.Init(DB); err != nil {
		log.Print("failed to connect to db:", err)
		os.Exit(1)
	}

	// create a new turbo app
	app := new(App)

	// create a new proxy
	prx := api.New(&api.Options{
		Key: Key,
		Url: Url,
	})

	// register api routes
	prx.Register(api.Routes)

	// set the proxy
	app.Proxy = prx

	// migrate the internals
	app.Migrate(
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

	// setup openai
	if err := ai.Set(Key, Url); err != nil {
		log.Print("Failed to setup AI", err)
		os.Exit(1)
	}

	// add middleware

	// with event logger
	hw := api.WithLogger(prx)
	// with auth / admin
	hw = api.WithAuth(hw)
	// with cors
	hw = api.WithCors(hw)

	// set the app handler
	app.Handler = hw

	return app
}
