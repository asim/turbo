#!/bin/bash

set -x
set -e

USER=test
PASS=Password1
FIRST_NAME=john
LAST_NAME=smith
KEY=xxx
ADDRESS=http://localhost:8080

function testApi {
	if [ -n $TOKEN ]; then
		curl -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -XPOST $ADDRESS$1 -d "$2"
	else
		curl -H "Content-Type: application/json" -XPOST $ADDRESS$1 -d "$2"
	fi

	if [ $? -eq 1 ]; then
		exit 1
	fi
}

echo "Starting proxy"

echo "Running tests"

if ! [ -f ./proxy-gpt ]; then
	go build -o ./proxy-gpt main.go
fi

# run proxy
API_KEY=$KEY ./proxy-gpt &
PID=$!

# sleep
sleep 1

# register a user
testApi /user/register "{\"username\": \"$USER\", \"password\": \"$PASS\", \"first_name\": \"$FIRST_NAME\", \"last_name\": \"$LAST_NAME\"}"

# login and set the token
RESPONSE=`testApi /auth/login "{\"username\": \"$USER\", \"password\": \"$PASS\"}"`
TOKEN=`jq '.Token' <<< $RESPONSE | cut -f 2 -d \"`

# create a chat
RESPONSE=`testApi /chat/create '{"name": "Example", "platform": "gpt-4"}'`
CHATID=`jq '.id' <<< $RESPONSE | cut -f 2 -d \"`

# delete the chat
testApi /chat/delete "{\"id\": \"$CHATID\"}"

# logout with the token
testApi /auth/logout "{}"

# cleanup
kill -9 $PID
rm -f ./proxy-gpt
