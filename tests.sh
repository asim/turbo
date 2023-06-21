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
	if [ ! -z "$TOKEN" ]; then
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

if ! [ -f ./turbo ]; then
	go build -o ./turbo main.go
fi

# run proxy
API_KEY=$KEY ./turbo &
PID=$!

# sleep
sleep 5

# signup a user
testApi /user/signup "{\"username\": \"$USER\", \"password\": \"$PASS\", \"first_name\": \"$FIRST_NAME\", \"last_name\": \"$LAST_NAME\"}"

# login and set the token
RESPONSE=`testApi /user/login "{\"username\": \"$USER\", \"password\": \"$PASS\"}"`
TOKEN=`jq '.Token' <<< $RESPONSE | cut -f 2 -d \"`

# create a chat
RESPONSE=`testApi /chat/create '{"name": "Example", "model": "gpt-3"}'`
CHATID=`jq '.id' <<< $RESPONSE | cut -f 2 -d \"`

# delete the chat
testApi /chat/delete "{\"id\": \"$CHATID\"}"

# logout with the token
testApi /user/logout "{}"

# cleanup
kill -9 $PID
rm -f ./turbo
