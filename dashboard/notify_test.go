package dashboard

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNotificationWakeupsAreCoalesced(t *testing.T) {
	s := NewState("eth0", nil)
	for i := 0; i < 100; i++ {
		s.SetActiveNotification(&Notification{Message: "hello", Theme: ThemeInfo})
	}
	select {
	case <-s.NotificationsChanged:
	default:
		t.Fatal("notification did not wake render loop")
	}
	select {
	case <-s.NotificationsChanged:
		t.Fatal("redundant wakeups queued")
	default:
	}
	s.SetActiveNotification(&Notification{Message: "old", ExpiresAt: time.Now().Add(-time.Second)})
	<-s.NotificationsChanged
	s.ClearExpiredNotification()
	if s.GetActiveNotification() != nil {
		t.Fatal("expired notification retained")
	}
	select {
	case <-s.NotificationsChanged:
	default:
		t.Fatal("expiry did not wake renderer")
	}
}

func TestNotificationRejectsOversizedInput(t *testing.T) {
	server, client := net.Pipe()
	s := &NotifyServer{state: &State{}}
	go s.handleConnection(server)
	defer client.Close()
	client.SetDeadline(time.Now().Add(time.Second))
	go client.Write([]byte(strings.Repeat("x", 4097)))
	reply, err := io.ReadAll(client)
	if err != nil || !strings.Contains(string(reply), "4096") {
		t.Fatalf("reply %q: %v", reply, err)
	}
}
