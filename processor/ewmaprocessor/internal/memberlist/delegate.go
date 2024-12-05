package memberlist

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/memberlist"
)

// Example taken from: https://github.com/octu0/example-memberlist/blob/master/05-broadcast/c.go

type MyDelegate struct {
	msgCh      chan []byte
	Broadcasts *memberlist.TransmitLimitedQueue
}

func NewMyDelegate(msgCh *chan []byte) *MyDelegate {
	return &MyDelegate{
		msgCh:      *msgCh,
		Broadcasts: new(memberlist.TransmitLimitedQueue),
	}
}

func (d *MyDelegate) NotifyMsg(msg []byte) {
	d.msgCh <- msg
}

func (d *MyDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.Broadcasts.GetBroadcasts(overhead, limit)
}

func (d *MyDelegate) NodeMeta(limit int) []byte {
	// not use, noop
	return []byte("")
}

func (d *MyDelegate) LocalState(join bool) []byte {
	// not use, noop
	return []byte("")
}

func (d *MyDelegate) MergeRemoteState(buf []byte, join bool) {
	// not use
}

type MyEventDelegate struct {
	Num int
}

func (d *MyEventDelegate) NotifyJoin(node *memberlist.Node) {
	d.Num += 1
}

func (d *MyEventDelegate) NotifyLeave(node *memberlist.Node) {
	d.Num -= 1
}

func (d *MyEventDelegate) NotifyUpdate(node *memberlist.Node) {
	// skip
}

func wait_signal(cancel context.CancelFunc) {
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGINT)
	for {
		select {
		case s := <-signal_chan:
			log.Printf("signal %s happen", s.String())
			cancel()
		}
	}
}
