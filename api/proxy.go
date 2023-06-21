package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	valid "github.com/asaskevich/govalidator"
	"github.com/asim/turbo/db"
	"github.com/asim/turbo/log"
	"github.com/google/uuid"
	"golang.org/x/crypto/acme/autocert"
	"gorm.io/gorm"
)

var (
	SessionCookie = "sess"

	// sessions
	sessMtx  sync.RWMutex
	sessions = map[string]*Session{}
)

type Options struct {
	// key for the url
	Key string
	// url to proxy to
	Url string
}

// Proxy handles all inbound requests
type Proxy struct {
	opts *Options
}

// Event is a request summary
type Event struct {
	// TODO: gorm validation
	gorm.Model
	ID        string        `json:"ID"`
	Timestamp time.Time     `json:"Timestamp"`
	ClientIP  string        `json:"ClientIP"`
	Endpoint  string        `json:"Endpoint"`
	Request   string        `json:"Request"`
	Response  string        `json:"Response"`
	Status    int           `json:"Status"`
	Message   string        `json:"Message"`
	Duration  time.Duration `json:"Duration"`
	Method    string        `json:"Method"`
	Params    string        `json:"Params"`
	UserID    string        `json:"UserID"`
}

// response wrapper for logger middleware
type response struct {
	status int
	size   int
	data   []byte
}

// response writer wrapper for logger middleware
type responseWriter struct {
	http.ResponseWriter // compose original http.ResponseWriter
	response            *response
}

func (r *responseWriter) Flush() {
	if v, ok := r.ResponseWriter.(http.Flusher); ok {
		v.Flush()
	}
}

func (r *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return r.ResponseWriter.(http.Hijacker).Hijack()
}

func (r *responseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b) // write response using original http.ResponseWriter
	// set size
	r.response.size += size
	// set data
	r.response.data = b

	if r.response.status == 0 {
		r.response.status = 200
	}

	return size, err
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode) // write status code using original http.ResponseWriter
	r.response.status = statusCode           // capture status code
}

func getIP(r *http.Request) string {
	if v := r.Header.Get("do-connecting-ip"); len(v) > 0 {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); len(v) > 0 {
		return v
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func newSession(user *User) (*Session, error) {
	// issue user session
	tk := uuid.New().String()
	sess := base64.StdEncoding.EncodeToString([]byte(tk))
	expiresAt := time.Now().Add(365 * 24 * time.Hour)

	// store the session
	session := &Session{
		Token:     sess,
		ExpiresAt: expiresAt,
		Username:  user.Username,
		UserID:    fmt.Sprintf("%v", user.ID),
	}

	// TODO: store sessions in redis
	if err := db.Create(session).Error; err != nil {
		log.Print("Failed to store session for", user.Username, err)
		return nil, err
	}

	// set in local sessions
	sessMtx.Lock()
	sessions[sess] = session
	sessMtx.Unlock()

	return session, nil
}

func getSession(tk string) (*Session, error) {
	// check session store
	sessMtx.RLock()
	sess, ok := sessions[tk]
	sessMtx.RUnlock()

	if ok {
		return sess, nil
	}

	// get session from the DB
	sess = new(Session)
	res := db.Where(`token = ?`, tk).First(sess)
	return sess, res.Error
}

func delSession(tk string) error {
	// delete from session map
	sessMtx.Lock()
	delete(sessions, tk)
	sessMtx.Unlock()

	// delete from db
	res := db.Where(`token = ?`, tk).Delete(&Session{})

	return res.Error
}

// respond as JSON or whatever else
func respond(w http.ResponseWriter, r *http.Request, vals interface{}) {
	// TODO: checkcontent type to decide how to respond
	w.Header().Set("Content-Type", "application/json")
	// TODO: check error
	b, _ := json.Marshal(vals)
	// write the response
	w.Write(b)
	// done?
}

func uri(api, ep string) string {
	return fmt.Sprintf("%s%s", api, ep)
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// All /v1/* calls are routed to OpenAI /v1/*
	// Everything else is routed internally
	if !strings.HasPrefix(r.URL.Path, "/v1/") {
		http.DefaultServeMux.ServeHTTP(w, r)
		return
	}

	// 1. get request
	// 2. make request
	// 3. return response
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	buf := bytes.NewReader(b)

	// TODO: check http path validity
	req, err := http.NewRequest(r.Method, uri(p.opts.Url, r.URL.Path), buf)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// TODO: Use/validate requested content-type
	req.Header.Set("Content-Type", "application/json")

	if len(p.opts.Key) > 0 {
		req.Header.Set("Authorization", "Bearer "+p.opts.Key)
	}
	// make request
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rsp.Body.Close()

	b, err = ioutil.ReadAll(rsp.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// return response to user
	w.Header().Set("Content-Type", "application/json")
	// write the response data
	w.Write(b)
	// set status code from proxy
	w.WriteHeader(rsp.StatusCode)
	// done?
}

func decode(r *http.Request, v interface{}) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		b = []byte(`{}`)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	_, err = valid.ValidateStruct(v)
	return err
}

// func contentType(r *http.Request) string {
// 	return r.Header.Get("Content-Type")
// }

func New(opts *Options) *Proxy {
	return &Proxy{
		opts: opts,
	}
}

func (p *Proxy) Register(routes map[string]http.HandlerFunc) {
	// TODO: use built in server mux
	for path, hdr := range routes {
		http.HandleFunc(path, hdr)
	}
}

func (p *Proxy) Run(address string, hd http.Handler) error {
	// serve tls
	if address == ":443" {
		m := &autocert.Manager{
			Cache:  autocert.DirCache(".turbo"),
			Prompt: autocert.AcceptTOS,
			Email:  "support@example.com",
		}
		s := &http.Server{
			Handler:   hd,
			Addr:      address,
			TLSConfig: m.TLSConfig(),
		}
		return s.ListenAndServeTLS("", "")
	}

	return http.ListenAndServe(address, hd)
}
