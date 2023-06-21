package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asim/turbo/db"
	"github.com/asim/turbo/log"
	"github.com/asim/turbo/util"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User session request to get the current user
type UserSessionRequest struct{}

type UserSessionResponse struct {
	User User `json:"user"`
}

// User who signs up
type User struct {
	// TODO: gorm validation
	gorm.Model
	ID        string  `json:"id" valid:"required"`
	FirstName string  `json:"first_name" valid:"length(1|30)"`
	LastName  string  `json:"last_name" valid:"length(1|30)"`
	Username  string  `json:"username" valid:"required,username,length(6|254)" gorm:"unique_index;not null"`
	Password  string  `json:"-"`
	Groups    []Group `json:"groups" gorm:"many2many:user_groups;"`
}

// UserIndexRequest for user/index
// TODO: worry about pagination later.
type UserIndexRequest struct {
	GroupID uint `json:"group_id" valid:"length(1|30)"`
}

// UserIndexResponse for user/index
type UserIndexResponse struct {
	Users []User `json:"users"`
}

// UserSignupRequest for user/register
// Need to create group from first+last name
type UserSignupRequest struct {
	// 	ID created in backend
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username" valid:"required,length(1|254)" gorm:"unique_index;not null"`
	Password  string `json:"password" valid:"required"`
	GroupName string `json:"group_name,omitempty"`
}

// UserSignupResponse for user/register
// same as authResponse
type UserSignupResponse struct {
	Token string `json:"Token"`
	User  User   `json:"User"`
}

// UserReadRequest for user/read
type UserReadRequest struct {
	ID string `json:"id" valid:"required"`
}

// UserReadResponse full user with id
// Response for user/create, user/read & user/update
type UserReadResponse struct {
	User User `json:"user"`
}

// UserUpdateRequest for user/update
// TODO: validate username and check is unique
type UserUpdateRequest struct {
	ID        string `json:"id" valid:"required"`
	FirstName string `json:"first_name" valid:"length(1|30)"`
	LastName  string `json:"last_name" valid:"length(1|30)"`
	Username  string `json:"username" valid:"required,username,length(6|254)"`
}

type UserUpdateResponse struct{}

// UserDeleteRequest for deleting
type UserDeleteRequest struct {
	ID string `json:"id" valid:"required"`
}

// UserDeleteResponse is empty as 200
type UserDeleteResponse struct{}

// UserLoginRequest struct for auth/login
type UserLoginRequest struct {
	Username string `json:"username" valid:"required,length(1|254)"` // currently username
	Password string `json:"password" valid:"required"`               // ^[a-z0-9@.-_+]+$ - no validation
}

// UserLoginResponse struct for auth/login
type UserLoginResponse struct {
	Token string `json:"Token"`
	User  User   `json:"User"`
}

// UserLogoutRequest struct for auth/logout
// Just need valid header
type UserLogoutRequest struct {
	Token string
}

// UserLogoutResponse struct for auth/logout
// Just need valid header
type UserLogoutResponse struct{}

// UserPasswordUpdateRequest is for updating a password by logged in user
type UserPasswordUpdateRequest struct {
	OldPassword string `json:"old_password" valid:"required"`
	NewPassword string `json:"new_password" valid:"required"`
}

type UserPasswordUpdateResponse struct{}

// The session for a given user
type Session struct {
	gorm.Model
	Token     string    `json:"token" gorm:"index"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	UserID    string    `json:"user_id"`
}

// UserLogin logs in a user using a username and password
func UserLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	lr := new(UserLoginRequest)
	lr.Username = r.Form.Get("username")
	lr.Password = r.Form.Get("password")

	if err := decode(r, lr); err != nil {
		http.Error(w, "Invalid login", http.StatusInternalServerError)
		return
	}

	// username is username
	username := lr.Username

	user := new(User)
	user.Username = lr.Username

	// check exists
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		http.Error(w, "Failed to login user", http.StatusInternalServerError)
		log.Print("Failed to login user", username, err)
		return
	}

	// compare passwords
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(lr.Password),
	); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		log.Print("Failed to login user", username, err)
		return
	}

	// create a new session
	sess, err := newSession(user)
	if err != nil {
		http.Error(w, "Failed to login", http.StatusInternalServerError)
		return
	}

	// lookup groups
	var groupIDs []GroupMember
	if err := db.Where("user_id = ?", user.ID).Find(&groupIDs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var ids []string
	for _, o := range groupIDs {
		ids = append(ids, o.GroupID)
	}

	var groups []Group
	if err := db.Where("id IN ?", ids).Find(&groups).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set user groups
	user.Groups = groups

	// set a session cookie
	http.SetCookie(w, &http.Cookie{
		Name:    SessionCookie,
		Value:   sess.Token,
		Expires: sess.ExpiresAt,
	})

	// success case
	redirectURL := r.Form.Get("redirect_url")
	if len(redirectURL) > 0 {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	respond(w, r, &UserLoginResponse{
		Token: sess.Token,
		User:  *user,
	})
}

// UserLogout logs out a user using a session token
func UserLogout(w http.ResponseWriter, r *http.Request) {
	lr := new(UserLogoutRequest)
	lr.Token = r.Form.Get("token")

	r.ParseForm()

	// TODO: make sure the user is logging out their own session

	if err := decode(r, lr); err != nil {
		http.Error(w, "Invalid request", http.StatusInternalServerError)
		return
	}

	// post form or json data
	if len(lr.Token) > 0 {
		delSession(lr.Token)
		return
	}

	// check the Authorization key header
	if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
		lr.Token = strings.TrimPrefix(v, "Bearer ")
	}

	// delete session token passed as bearer token
	if len(lr.Token) > 0 {
		delSession(lr.Token)
		return
	}

	// get the session cookie
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		if err == http.ErrNoCookie {
			// If the cookie is not set, return an unauthorized status
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// For any other type of error, return a bad request status
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(c.Value) == 0 {
		return
	}

	// delete from db
	delSession(c.Value)
}

// UserPasswordUpdate updates the password of a user
func UserPasswordUpdate(w http.ResponseWriter, r *http.Request) {
	// since /auth/password/update is excluded from logging/auth
	// we need to call the authenticate function ourselves
	authenticate(w, r, http.HandlerFunc(authPasswordUpdate))
}

func authPasswordUpdate(w http.ResponseWriter, r *http.Request) {
	// 1. check username
	// 2. reset password
	// 3. send username
	r.ParseForm()

	ua := new(UserPasswordUpdateRequest)
	ua.NewPassword = r.Form.Get("new_password")
	ua.OldPassword = r.Form.Get("old_password")

	if err := decode(r, ua); err != nil {
		http.Error(w, "Invalid request", http.StatusInternalServerError)
		return
	}

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// get the user
	var user User
	user.ID = sess.UserID

	if err := db.First(&user).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// compare passwords
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(ua.OldPassword),
	); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		log.Print("Failed to change password for", user.Username, err)
		return
	}

	// update the password
	// salt password
	hashedPw, err := util.GetHash(ua.NewPassword)
	if err != nil {
		http.Error(w, "Failed to to hash password", http.StatusInternalServerError)
		return
	}

	// set user password
	user.Password = hashedPw

	// save user
	// call update
	if err := db.Update(&user).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respond(w, r, &UserPasswordUpdateResponse{})
}

// UserSession reads the current user session and returns the user
func UserSession(w http.ResponseWriter, r *http.Request) {
	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var user User
	user.ID = sess.UserID

	if err := db.First(&user).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// lookup groups
	var groupIDs []GroupMember
	if err := db.Where("user_id = ?", user.ID).Find(&groupIDs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var ids []string
	for _, o := range groupIDs {
		ids = append(ids, o.GroupID)
	}

	var groups []Group
	if err := db.Where("id IN ?", ids).Find(&groups).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set user groups
	user.Groups = groups

	respond(w, r, &UserSessionResponse{
		User: user,
	})
}

func UserRead(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ur := new(UserReadRequest)
	ur.ID = r.Form.Get("id")

	if err := decode(r, ur); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	// must be the user to get the info
	if sess.UserID != ur.ID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var user User
	user.ID = ur.ID

	if err := db.First(&user).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// lookup groups
	var groupIDs []GroupMember
	if err := db.Where("user_id = ?", user.ID).Find(&groupIDs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var ids []string
	for _, o := range groupIDs {
		ids = append(ids, o.GroupID)
	}

	var groups []Group
	if err := db.Where("id IN ?", ids).Find(&groups).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set user groups
	user.Groups = groups

	respond(w, r, &UserReadResponse{
		User: user,
	})
}

func UserSignup(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ur := new(UserSignupRequest)
	ur.FirstName = r.Form.Get("first_name")
	ur.LastName = r.Form.Get("last_name")
	ur.Username = r.Form.Get("username")
	ur.Password = r.Form.Get("password")
	ur.GroupName = r.Form.Get("group_name")

	if err := decode(r, ur); err != nil {
		http.Error(w, "Invalid request", http.StatusInternalServerError)
		return
	}

	// create a user
	user := &User{
		ID:        uuid.New().String(),
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Username:  ur.Username,
		Password:  ur.Password,
	}

	// check exists
	var err error
	user, err = CreateUser(user)
	if err != nil {
		http.Error(w, "Failed to signup user: "+err.Error(), http.StatusInternalServerError)
		log.Print("Failed to signup", ur.Username, err.Error())
		return
	}

	// TODO: verify the username address

	// We've generated a user, now we create their own
	// This is new registration flow, meaning the user
	// is signing up themselves and generating a new group
	// In the event they're joining an existing or that's
	// an invite to the group, not new user registration

	// if no group name use personal
	if len(ur.GroupName) == 0 {
		ur.GroupName = "Personal"
	}

	// Create new group
	group := Group{
		ID:      uuid.New().String(),
		Name:    ur.GroupName,
		OwnerID: user.ID,
	}

	// Save group to database
	if err := db.Create(&group).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// great, we have an group!
	user.Groups = append(user.Groups, group)

	// create group member
	if err := AddUserToGroup(&GroupMember{
		GroupID: group.ID,
		UserID:  group.OwnerID,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// create a new session
	sess, err := newSession(user)
	if err != nil {
		http.Error(w, "Registration complete. Login failed.", http.StatusInternalServerError)
		return
	}

	// set a session cookie
	http.SetCookie(w, &http.Cookie{
		Name:    SessionCookie,
		Value:   sess.Token,
		Expires: sess.ExpiresAt,
	})

	// success case
	redirectURL := r.Form.Get("redirect_url")
	if len(redirectURL) > 0 {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// success
	respond(w, r, &UserSignupResponse{
		Token: sess.Token,
		User:  *user,
	})
}

func UserUpdate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ur := new(UserUpdateRequest)
	ur.ID = r.Form.Get("id")
	ur.FirstName = r.Form.Get("first_name")
	ur.LastName = r.Form.Get("last_name")
	ur.Username = r.Form.Get("username")

	if err := decode(r, ur); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	// must be the user to get the info
	if sess.UserID != ur.ID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user := new(User)
	user.ID = ur.ID

	if err := db.First(user).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set the following fields
	user.FirstName = ur.FirstName
	user.LastName = ur.LastName
	user.Username = ur.Username

	// call update
	if err := db.Update(user).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respond(w, r, &UserUpdateResponse{})
}

func AddUserToGroup(om *GroupMember) error {
	if err := db.Create(&om).Error; err != nil {
		return err
	}

	return nil
}

func CreateUser(ur *User) (*User, error) {
	if len(ur.FirstName) == 0 {
		// set name as first part of username
		parts := strings.Split(ur.Username, "@")
		ur.FirstName = parts[0]
	}

	// generate a new password
	if len(ur.Password) == 0 {
		ur.Password = util.Password(10)
	}

	// salt password
	hashedPw, err := util.GetHash(ur.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password")
	}

	if len(ur.ID) == 0 {
		ur.ID = uuid.New().String()
	}

	// create a user
	user := &User{
		ID:        ur.ID,
		FirstName: ur.FirstName,
		LastName:  ur.LastName,
		Username:  ur.Username,
		Password:  hashedPw,
	}

	// check exists
	if err := db.Where("username = ?", user.Username).First(&User{}).Error; err == nil {
		return nil, fmt.Errorf("User exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// write the db record
	if err := db.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func GetUser(username string) (User, error) {
	var user User

	// check exists
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return user, err
	}

	return user, nil
}

func GetUsers(ids []string) ([]User, error) {
	var users []User

	if err := db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}
