package memberlist

import (
	"encoding/json"

	"github.com/hashicorp/memberlist"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// represent the type of action the member took
type AnomalyAction int

const (
	AnomalyActionNone AnomalyAction = iota
	AnomalyActionStart
	AnomalyActionStop
)

// AnomalyMessage is a struct that represents a message that is broadcasted
// to all members of the memberlist.
type AnomalyMessage struct {
	AnomalyAction AnomalyAction   `json:"action"`
	TraceID       pcommon.TraceID `json:"trace_id"`
	SpanID        pcommon.SpanID  `json:"span_id"`
	GroupKey      string          `json:"group_key"`
}

func (m AnomalyMessage) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (m AnomalyMessage) Finished() {
	// nop
}

func (m AnomalyMessage) Message() []byte {
	data, err := json.Marshal(m)
	if err != nil {
		return []byte("")
	}
	return data
}

func ParseMyBroadcastMessage(data []byte) (*AnomalyMessage, bool) {
	msg := new(AnomalyMessage)
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false
	}
	return msg, true
}
