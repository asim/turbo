package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type User struct {
	ID        uint64 `gorm:"primaryKey"`
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt DeletedAt `gorm:"index"`
}

type Session struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64
	Token     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt DeletedAt `gorm:"index"`
}

func TestDB(t *testing.T) {
	// Initialize the database with an empty address, which defaults to SQLite.
	err := Init("")
	assert.NoError(t, err)

	// Perform migrations.
	Migrate(&User{}, &Session{})

	// Test creating a new user.
	user := User{Name: "Alice"}
	result := Create(&user)
	assert.NoError(t, result.Error)
	assert.Equal(t, uint64(1), user.ID)

	// Test finding the user.
	foundUser := &User{}
	result = First(foundUser, user.ID)
	assert.NoError(t, result.Error)
	assert.Equal(t, user.Name, foundUser.Name)

	// Test updating the user.
	user.Name = "Bob"
	result = Update(&user)
	assert.NoError(t, result.Error)

	// Test deleting the user.
	result = Delete(&user)
	assert.NoError(t, result.Error)

	// Test finding all users.
	var users []User
	result = Find(&users)
	assert.NoError(t, result.Error)
	assert.Empty(t, users)

	// Test creating a new session.
	session := Session{UserID: user.ID, Token: "token123"}
	result = Create(&session)
	assert.NoError(t, result.Error)
	assert.Equal(t, uint64(1), session.ID)

	// Test finding the session.
	foundSession := &Session{}
	result = First(foundSession, session.ID)
	assert.NoError(t, result.Error)
	assert.Equal(t, session.Token, foundSession.Token)

	// Test deleting the session.
	result = Delete(&session)
	assert.NoError(t, result.Error)
}
