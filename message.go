package main

import (
	"encoding/json"
	"log"
)

const SendMessageAction = "send_message"
const JoinChannelAction = "join_channel"
const LeaveChannelAction = "leave_channel"
const MemberAddedAction = "member_added"
const MemberRemovedAction = "member_removed"
const JoinChannelPrivateAction = "join_channel_private"
const ChannelJoinedAction = "channel_joined"

type Message struct {
	Action string   `json:"action"`
	Event  string   `json:"event"`
	Name   string   `json:"name"`
	Data   []byte   `json:"data"`
	Target *Channel `json:"target"`
	Sender *Client  `json:"sender"`
}

func (message *Message) encode() []byte {
	json, err := json.Marshal(message)
	if err != nil {
		log.Println(err)
	}

	return json
}
