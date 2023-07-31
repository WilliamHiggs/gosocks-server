package main

import (
	"encoding/json"
	"log"
)

const SendMessageAction = "send-message"
const JoinChannelAction = "join-channel"
const LeaveChannelAction = "leave-channel"
const UserJoinedAction = "user-join"
const UserLeftAction = "user-left"
const JoinChannelPrivateAction = "join-channel-private"
const ChannelJoinedAction = "channel-joined"

type Message struct {
	Action  string   `json:"action"`
	Message string   `json:"message"`
	Target  *Channel `json:"target"`
	Sender  *Client  `json:"sender"`
}

func (message *Message) encode() []byte {
	json, err := json.Marshal(message)
	if err != nil {
		log.Println(err)
	}

	return json
}
