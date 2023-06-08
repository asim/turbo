package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/asim/proxy-gpt/db"
	"github.com/asim/proxy-gpt/event"
	"github.com/asim/proxy-gpt/log"
	"github.com/google/uuid"
)

var (
	Routes = map[string]http.HandlerFunc{
		// register the chat api
		"/chat/create":      ChatCreate,
		"/chat/read":        ChatRead,
		"/chat/update":      ChatUpdate,
		"/chat/delete":      ChatDelete,
		"/chat/prompt":      ChatPrompt,
		"/chat/index":       ChatIndex,
		"/chat/stream":      ChatStream,
		"/chat/user/add":    ChatUserAdd,
		"/chat/user/remove": ChatUserRemove,

		// team apis
		"/team/create":         TeamCreate,
		"/team/delete":         TeamDelete,
		"/team/read":           TeamRead,
		"/team/update":         TeamUpdate,
		"/team/index":          TeamIndex,
		"/team/members":        TeamMembers,
		"/team/members/add":    TeamMembersAdd,
		"/team/members/remove": TeamMembersRemove,

		// register a user apis
		"/user/signup":          UserSignup,
		"/user/login":           UserLogin,
		"/user/logout":          UserLogout,
		"/user/read":            UserRead,
		"/user/update":          UserUpdate,
		"/user/session":         UserSession,
		"/user/password/update": UserPasswordUpdate,
	}
)

var (
	// Excludes paths from authentication / logging request-response
	Excludes = []string{
		"/user/signup",
		"/user/login",
		"/user/logout",
		"/user/password/update",
	}
)

// WithCors returns cors setting middleware
func WithCors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set cors origin allow all
		SetHeaders(w, r)

		// if options return immediately
		if r.Method == "OPTIONS" {
			return
		}

		h.ServeHTTP(w, r)
	})
}

// WithAdmin enables basic auth for an admin system
func WithAdmin(user, pass string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request has basic auth credentials
			username, password, ok := r.BasicAuth()
			if !ok || username != user || password != pass {
				unauthorized(w)
				return
			}

			// Call the wrapped handler if the credentials are valid
			h.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("401 Unauthorized\n"))
}

func authenticate(w http.ResponseWriter, r *http.Request, h http.Handler) {
	var tk string

	// check the Authorization key header
	if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
		tk = strings.TrimPrefix(v, "Bearer ")
	}

	// check websocket for token
	if isWebSocket(r) {
		r.ParseForm()

		if v := r.Form.Get("token"); v != "null" {
			tk = v
		}
	}

	// check cookie for valid session
	if len(tk) == 0 {
		c, err := r.Cookie(SessionCookie)
		if err == nil && len(c.Value) > 0 {
			tk = c.Value
		} else if _, password, ok := r.BasicAuth(); ok {
			// set the token as the password
			tk = password
		} else if r.URL.User != nil {
			// try the url password as token
			tk, _ = r.URL.User.Password()
		}
	}

	// check session exists
	if len(tk) > 0 {
		// we have a token, get the session for it
		sess, err := getSession(tk)
		if err != nil {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		if !sess.ExpiresAt.After(time.Now()) {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			// TODO: delete the session
			go delSession(tk)
			return
		}

		// add user session to context
		ctx := context.WithValue(r.Context(), Session{}, sess)
		req := r.Clone(ctx)

		// valid token, serve it
		h.ServeHTTP(w, req)
		return
	}

	// no token found
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

// WithAuth will enable auth via authorization header, sess cookie or basic auth token
func WithAuth(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// do not authenticate our excludes
		for _, path := range Excludes {
			if strings.Compare(r.URL.Path, path) == 0 {
				h.ServeHTTP(w, r)
				return
			}
		}

		authenticate(w, r, h)
	}

	return http.HandlerFunc(fn)
}

// WithLogger will log the events
func WithLogger(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// read body
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// reset body
		r.Body = ioutil.NopCloser(bytes.NewReader(b))

		// log the event
		ev := &Event{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			ClientIP:  getIP(r),
			Endpoint:  r.URL.Path,
			Request:   string(b),
		}

		// craft a response
		rsp := &response{}

		rw := responseWriter{
			ResponseWriter: w, // compose original http.ResponseWriter
			response:       rsp,
		}

		// serve standard handler
		h.ServeHTTP(&rw, r)

		// record the duration
		ev.Duration = time.Since(start)
		ev.Status = rsp.status
		ev.Response = string(rsp.data)
		// some extra info
		ev.Method = r.Method
		ev.Params = r.URL.Query().Encode()

		// attempt to pull user session from context
		sess, ok := r.Context().Value(Session{}).(*Session)
		if ok {
			// set the user id in the event
			ev.UserID = sess.UserID
		}

		// log the event to stdout
		// do not log request/response
		// but do save in the database
		log.WithFields(log.Fields{
			"id":        ev.ID,
			"client_ip": ev.ClientIP,
			"endpoint":  ev.Endpoint,
			"status":    ev.Status,
			"message":   ev.Message,
			"duration":  ev.Duration,
			"method":    ev.Method,
			"params":    ev.Params,
			"user_id":   ev.UserID,
		}).Println("request")

		// WARNING WARNING DANGER DANGER
		// don't log request/response for sensitive data
		for _, path := range Excludes {
			if strings.Compare(r.URL.Path, path) == 0 {
				// strip request/response from event
				ev.Request = ""
				ev.Response = ""
				break
			}
		}

		// write the event to db
		// async to avoid slowdown
		go db.Create(ev)

		// publish the event
		go event.Publish("events", ev)
	}

	return http.HandlerFunc(fn)
}
