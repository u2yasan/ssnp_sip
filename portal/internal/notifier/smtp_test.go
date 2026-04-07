package notifier

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSMTPNotifierSendSuccess(t *testing.T) {
	server := newSMTPTestServer(t, smtpTestOptions{startTLSSupported: true, authOK: true})
	defer server.Close()

	n := SMTPNotifier{
		Host:     server.Host(),
		Port:     server.Port(),
		Username: "user",
		Password: "secret",
		From:     "ssnp@example.invalid",
		Timeout:  5 * time.Second,
		RootCAs:  server.RootCAs(),
	}
	notification := Notification{
		NodeID:     "node-abc",
		AlertCode:  "portal_unreachable",
		Severity:   SeverityWarning,
		Channel:    "email",
		Recipient:  "ops@example.invalid",
		OccurredAt: "2026-04-07T10:00:00Z",
		Message:    "Program Agent cannot reach portal",
	}
	if err := n.Send(context.Background(), notification); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	got := server.LastMessage()
	if got.Recipient != "ops@example.invalid" {
		t.Fatalf("recipient = %q, want ops@example.invalid", got.Recipient)
	}
	if !strings.Contains(got.Data, "Subject: [SSNP][WARNING] portal_unreachable node-abc") {
		t.Fatalf("message missing subject: %s", got.Data)
	}
	if !strings.Contains(got.Data, "dedupe/cooldown may suppress repeats") {
		t.Fatalf("message missing note: %s", got.Data)
	}
}

func TestSMTPNotifierRejectsServerWithoutStartTLS(t *testing.T) {
	server := newSMTPTestServer(t, smtpTestOptions{startTLSSupported: false, authOK: true})
	defer server.Close()

	n := SMTPNotifier{
		Host:     server.Host(),
		Port:     server.Port(),
		Username: "user",
		Password: "secret",
		From:     "ssnp@example.invalid",
		Timeout:  5 * time.Second,
		RootCAs:  server.RootCAs(),
	}
	err := n.Send(context.Background(), Notification{
		NodeID:     "node-abc",
		AlertCode:  "portal_unreachable",
		Severity:   SeverityWarning,
		Channel:    "email",
		Recipient:  "ops@example.invalid",
		OccurredAt: "2026-04-07T10:00:00Z",
		Message:    "Program Agent cannot reach portal",
	})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("Send() error = %v, want STARTTLS failure", err)
	}
}

func TestSMTPNotifierAuthFailure(t *testing.T) {
	server := newSMTPTestServer(t, smtpTestOptions{startTLSSupported: true, authOK: false})
	defer server.Close()

	n := SMTPNotifier{
		Host:     server.Host(),
		Port:     server.Port(),
		Username: "user",
		Password: "wrong",
		From:     "ssnp@example.invalid",
		Timeout:  5 * time.Second,
		RootCAs:  server.RootCAs(),
	}
	err := n.Send(context.Background(), Notification{
		NodeID:     "node-abc",
		AlertCode:  "portal_unreachable",
		Severity:   SeverityWarning,
		Channel:    "email",
		Recipient:  "ops@example.invalid",
		OccurredAt: "2026-04-07T10:00:00Z",
		Message:    "Program Agent cannot reach portal",
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "auth") {
		t.Fatalf("Send() error = %v, want auth failure", err)
	}
}

func TestSMTPNotifierRejectsInvalidRecipient(t *testing.T) {
	n := SMTPNotifier{
		Host:     "smtp.example.invalid",
		Port:     587,
		Username: "user",
		Password: "secret",
		From:     "ssnp@example.invalid",
		Timeout:  5 * time.Second,
	}
	err := n.Send(context.Background(), Notification{
		NodeID:     "node-abc",
		AlertCode:  "portal_unreachable",
		Severity:   SeverityWarning,
		Channel:    "email",
		Recipient:  "not-an-email",
		OccurredAt: "2026-04-07T10:00:00Z",
		Message:    "Program Agent cannot reach portal",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid recipient address") {
		t.Fatalf("Send() error = %v, want invalid recipient failure", err)
	}
}

type smtpTestOptions struct {
	startTLSSupported bool
	authOK            bool
}

type smtpCapturedMessage struct {
	Recipient string
	Data      string
}

type smtpTestServer struct {
	listener net.Listener
	options  smtpTestOptions
	host     string
	port     int
	tlsCert  tls.Certificate
	certPEM  []byte
	mu       sync.Mutex
	last     smtpCapturedMessage
}

func newSMTPTestServer(t *testing.T, options smtpTestOptions) *smtpTestServer {
	t.Helper()
	cert, certPEM := makeTLSCert(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	server := &smtpTestServer{
		listener: ln,
		options:  options,
		host:     addr.IP.String(),
		port:     addr.Port,
		tlsCert:  cert,
		certPEM:  certPEM,
	}
	go server.serve(t)
	return server
}

func (s *smtpTestServer) Host() string { return s.host }
func (s *smtpTestServer) Port() int    { return s.port }
func (s *smtpTestServer) RootCAs() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(s.certPEM)
	return pool
}

func (s *smtpTestServer) Close() {
	_ = s.listener.Close()
}

func (s *smtpTestServer) LastMessage() smtpCapturedMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func (s *smtpTestServer) serve(t *testing.T) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(t, conn)
	}
}

func (s *smtpTestServer) handleConn(t *testing.T, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeSMTPLine(t, writer, "220 localhost ESMTP ready")

	tlsActive := false
	recipient := ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("ReadString() error = %v", err)
		}
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(strings.ToUpper(line), "EHLO"), strings.HasPrefix(strings.ToUpper(line), "HELO"):
			if s.options.startTLSSupported && !tlsActive {
				writeSMTPMultiLine(t, writer, []string{"250-localhost", "250-STARTTLS", "250 AUTH PLAIN"})
			} else {
				writeSMTPMultiLine(t, writer, []string{"250-localhost", "250 AUTH PLAIN"})
			}
		case strings.EqualFold(line, "STARTTLS"):
			if !s.options.startTLSSupported {
				writeSMTPLine(t, writer, "454 TLS not available")
				return
			}
			writeSMTPLine(t, writer, "220 Ready to start TLS")
			tlsConn := tls.Server(conn, &tls.Config{
				Certificates: []tls.Certificate{s.tlsCert},
				MinVersion:   tls.VersionTLS12,
			})
			if err := tlsConn.Handshake(); err != nil {
				t.Fatalf("Handshake() error = %v", err)
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
			tlsActive = true
		case strings.HasPrefix(strings.ToUpper(line), "AUTH PLAIN"):
			if !tlsActive {
				writeSMTPLine(t, writer, "538 Encryption required")
				return
			}
			if !s.options.authOK {
				writeSMTPLine(t, writer, "535 Authentication failed")
				return
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "AUTH PLAIN"))
			if payload == "" {
				writeSMTPLine(t, writer, "334")
				encoded, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("ReadString(auth) error = %v", err)
				}
				payload = strings.TrimSpace(encoded)
			}
			if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
				t.Fatalf("DecodeString() error = %v", err)
			}
			writeSMTPLine(t, writer, "235 Authentication successful")
		case strings.HasPrefix(strings.ToUpper(line), "MAIL FROM:"):
			writeSMTPLine(t, writer, "250 OK")
		case strings.HasPrefix(strings.ToUpper(line), "RCPT TO:"):
			recipient = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "RCPT TO:")), "<>")
			writeSMTPLine(t, writer, "250 OK")
		case strings.EqualFold(line, "DATA"):
			writeSMTPLine(t, writer, "354 End data with <CR><LF>.<CR><LF>")
			var dataLines []string
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("ReadString(data) error = %v", err)
				}
				if strings.TrimSpace(dataLine) == "." {
					break
				}
				dataLines = append(dataLines, dataLine)
			}
			s.mu.Lock()
			s.last = smtpCapturedMessage{
				Recipient: recipient,
				Data:      strings.Join(dataLines, ""),
			}
			s.mu.Unlock()
			writeSMTPLine(t, writer, "250 OK")
		case strings.EqualFold(line, "QUIT"):
			writeSMTPLine(t, writer, "221 Bye")
			return
		default:
			writeSMTPLine(t, writer, "250 OK")
		}
	}
}

func writeSMTPLine(t *testing.T, writer *bufio.Writer, line string) {
	t.Helper()
	if _, err := writer.WriteString(line + "\r\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

func writeSMTPMultiLine(t *testing.T, writer *bufio.Writer, lines []string) {
	t.Helper()
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			t.Fatalf("WriteString() error = %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

func makeTLSCert(t *testing.T) (tls.Certificate, []byte) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair() error = %v", err)
	}
	return cert, certPEM
}
