package common

import (
	"net"
	"testing"
	"time"
)

func TestSendEmailImplicitTLSHasBoundedTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	previousServer := SMTPServer
	previousPort := SMTPPort
	previousSSL := SMTPSSLEnabled
	previousAccount := SMTPAccount
	previousFrom := SMTPFrom
	previousToken := SMTPToken
	previousTimeout := smtpSendTimeout
	t.Cleanup(func() {
		SMTPServer = previousServer
		SMTPPort = previousPort
		SMTPSSLEnabled = previousSSL
		SMTPAccount = previousAccount
		SMTPFrom = previousFrom
		SMTPToken = previousToken
		smtpSendTimeout = previousTimeout
	})

	SMTPServer = "127.0.0.1"
	SMTPPort = listener.Addr().(*net.TCPAddr).Port
	SMTPSSLEnabled = true
	SMTPAccount = "sender@example.com"
	SMTPFrom = "sender@example.com"
	SMTPToken = "test-token"
	smtpSendTimeout = 50 * time.Millisecond

	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			close(accepted)
			time.Sleep(300 * time.Millisecond)
			_ = conn.Close()
		}
	}()

	started := time.Now()
	err = SendEmail("subject", "recipient@example.com", "content")
	elapsed := time.Since(started)
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("SMTP connection was not attempted")
	}
	if err == nil {
		t.Fatal("SendEmail returned nil for an unresponsive SMTP server")
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("SendEmail exceeded timeout: %s", elapsed)
	}
}
