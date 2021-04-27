package main

import (
	"time"
)

type BitProphetBot struct {
	ServiceChannel chan *bpServiceEvent
}

func CreateBitProphetBot() *BitProphetBot {
	return &BitProphetBot{
		ServiceChannel: make(chan *bpServiceEvent, 0),
	}
}

func (b *BitProphetBot) Run() {
	b.ServiceChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "BOT",
		EventData: "[BitProphetBot::Run] Started Bot",
	}
	autoSuggestTicker := time.NewTicker(60 * time.Second)
	for {
		select {
		case <-autoSuggestTicker.C:
			{
				b.ServiceChannel <- &bpServiceEvent{
					Time:      time.Now(),
					EventType: "BOT",
					EventData: "[BitProphetBot::Run] Running AutoSuggest",
				}
			}
		}
	}
}
