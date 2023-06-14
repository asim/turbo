# Admin 

Some helper tools

## Usage

Requires `DB_ADDRESS` env var for postgres usage (postgres://host:address/database string)

### Commands

```
create - create a user
user - get a user by id
list - list all the users
chat - get a chat by id
chatUsers - get the chat users
messages - list messages in a chat
deleteMessage - delete a messsage
reset - reset username/password
```

### Help

```
# list users
admin list

# create a user
admin create [username] [password]

# get user by id
admin user [userID]

# get a chat by id
admin chat [chatID]

# get chat users by chat id
admin chatUsers [chatID]

# get messages by chat id
admin messages [chatID]

# delete a message by id
admin deleteMessage [messageID]

# reset password
admin reset [username] [password]
```
