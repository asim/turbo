package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/asim/proxy-gpt/api"
	"github.com/asim/proxy-gpt/db"
	"github.com/asim/proxy-gpt/util"
)

var (
	// The backend database to connect to
	// Database = flag.String("database", "", "")
	Database = os.Getenv("DB_ADDRESS")
)

func GetChat(id string) (api.Chat, error) {
	chat, err := api.GetChat(id)
	if err != nil {
		return api.Chat{}, err
	}
	return *chat, nil
}

func GetUser(id string) (api.User, error) {
	var user api.User
	user.ID = id

	if err := db.First(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}

func ListUsers() ([]api.User, error) {
	var users []api.User

	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func ListChatUsers(chatID string) ([]api.User, error) {
	chatUsers, err := api.GetChatUsers(chatID)
	if err != nil {
		return nil, err
	}

	var ids []string
	fmt.Println("got", chatUsers)
	for _, id := range chatUsers {
		ids = append(ids, id.UserID)
	}
	return api.GetUsers(ids)
}

func ListMessages(chatID string) ([]api.Message, error) {
	var messages []api.Message

	// get messages
	res := db.Where("chat_id = ?", chatID).Order("created_at").Find(&messages)
	if err := res.Error; err != nil {
		return nil, err
	}

	return messages, nil
}

func DeleteMessage(id string) error {
	var msg api.Message
	msg.ID = id
	return db.Where("id = ?", id).Delete(&msg).Error
}

func ResetPassword(username, password string) error {
	if len(username) == 0 || len(password) == 0 {
		return fmt.Errorf("missing username or password")
	}

	user := new(api.User)
	user.Username = username

	// check exists
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return err
	}

	pw, err := util.GetHash(password)
	if err != nil {
		return err
	}

	// reset password
	user.Password = pw

	fmt.Println(user.Username, user.Password)

	// write the user
	if err := db.Update(&user).Error; err != nil {
		return err
	}

	return nil
}

func CreateUser(username, password string) error {
	if len(username) == 0 || len(password) == 0 {
		return fmt.Errorf("missing username or password")
	}

	user := new(api.User)
	user.Username = username

	// check exists
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		pw, err := util.GetHash(password)
		if err != nil {
			return err
		}

		user.Password = pw

		fmt.Println(user.Username, user.Password)

		// write the user
		if err := db.Update(&user).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("user already exist")
	}

	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()

	// initialise db connection
	// if err := db.Init(*Database); err != nil {
	if err := db.Init(Database); err != nil {
		fmt.Println(err)
		return
	}

	usage := "admin {create|list|reset|user|messages|chatUsers|deleteMessage}"

	// return
	if len(args) == 0 {
		fmt.Println(usage)
		return
	}

	switch args[0] {
	case "user":
		id := args[1]
		user, err := GetUser(id)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(user.ID, user.Username)
	case "list":
		users, err := ListUsers()
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, user := range users {
			fmt.Println(user.ID, user.Username, user.Password)
		}
	case "chat":
		chat, err := GetChat(args[1])
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(chat.ID, chat.Name, chat.UserID, chat.GroupID)
	case "chatUsers":
		id := args[1]
		users, err := ListChatUsers(id)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, user := range users {
			fmt.Println(user.Username, user.Password)
		}
	case "messages":
		id := args[1]
		messages, err := ListMessages(id)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, message := range messages {
			fmt.Println(message.ID, message.Prompt, message.Reply)
		}
	case "deleteMessage":
		id := args[1]
		err := DeleteMessage(id)
		if err != nil {
			fmt.Println(err)
			return
		}
	case "reset":
		// strip command
		args = args[1:]

		// check arg length
		if len(args) != 2 {
			fmt.Println("Missing username and password")
			return
		}

		if err := ResetPassword(args[0], args[1]); err != nil {
			fmt.Println(err)
			return
		}
	case "create":
		// strip command
		args = args[1:]

		// check arg length
		if len(args) != 2 {
			fmt.Println("Missing username and password")
			return
		}

		if err := CreateUser(args[0], args[1]); err != nil {
			fmt.Println(err)
			return
		}
	default:
		fmt.Println(usage)
		return
	}
}
