package email

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// selfSignedCert returns a TLS certificate for "localhost" and a pool that trusts it
func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		DNSNames:              []string{"localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

// fakeSMTP speaks just enough SMTP for one message; implicitTLS mimics port 465, otherwise it offers STARTTLS
func fakeSMTP(t *testing.T, cert tls.Certificate, implicitTLS bool, offerStartTLS bool) (port int, got chan string) {
	return fakeSMTPWithAuth(t, cert, implicitTLS, offerStartTLS, "PLAIN")
}

func fakeSMTPWithAuth(t *testing.T, cert tls.Certificate, implicitTLS bool, offerStartTLS bool, authMech string) (port int, got chan string) {
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		if implicitTLS {
			conn = tls.Server(conn, tlsCfg)
		}
		defer conn.Close()
		r, w := bufio.NewReader(conn), conn
		send := func(s string) { w.Write([]byte(s + "\r\n")) }
		send("220 localhost ESMTP fake")
		tlsActive := implicitTLS
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"):
				if !tlsActive && offerStartTLS {
					send("250-localhost\r\n250 STARTTLS")
				} else {
					send("250-localhost\r\n250 AUTH " + authMech)
				}
			case cmd == "STARTTLS":
				send("220 go ahead")
				tc := tls.Server(conn, tlsCfg)
				conn, r, w = tc, bufio.NewReader(tc), tc
				tlsActive = true
			case strings.HasPrefix(cmd, "AUTH PLAIN"):
				if !strings.Contains(authMech, "PLAIN") {
					send("504 5.7.4 Unrecognized authentication type")
					continue
				}
				data.WriteString("[auth-over-tls=" + map[bool]string{true: "yes", false: "no"}[tlsActive] + "]\n")
				send("235 ok")
			case strings.HasPrefix(cmd, "AUTH LOGIN"):
				send("334 VXNlcm5hbWU6") // "Username:"
				if _, err := r.ReadString('\n'); err != nil {
					return
				}
				send("334 UGFzc3dvcmQ6") // "Password:"
				if _, err := r.ReadString('\n'); err != nil {
					return
				}
				data.WriteString("[auth-over-tls=" + map[bool]string{true: "yes", false: "no"}[tlsActive] + "]\n")
				send("235 2.7.0 Authentication successful")
			case strings.HasPrefix(cmd, "MAIL FROM"), strings.HasPrefix(cmd, "RCPT TO"):
				send("250 ok")
			case cmd == "DATA":
				send("354 send")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					data.WriteString(l)
				}
				send("250 queued")
				got <- data.String()
			case cmd == "QUIT":
				send("221 bye")
				return
			default:
				send("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func newTestService(port int, secure bool, pool *x509.CertPool) *Service {
	return &Service{host: "localhost", port: port, username: "mailer@example.com", password: "pw",
		from: "no-reply@example.com", fromName: "Virtual Tours", secure: secure, rootCAs: pool}
}

func checkMessage(t *testing.T, msg string) {
	t.Helper()
	for _, want := range []string{"[auth-over-tls=yes]", "From: \"Virtual Tours\" <no-reply@example.com>", "To: user@example.com",
		"Subject: Your Virtual Tours verification code", "Date: ", "Message-ID: <", "    123456"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestSendOTPWithSTARTTLS(t *testing.T) { // like nodemailer secure:false on 587
	cert, pool := selfSignedCert(t)
	port, got := fakeSMTP(t, cert, false, true)
	if err := newTestService(port, false, pool).SendOTP("user@example.com", "123456", "verify_email"); err != nil {
		t.Fatal(err)
	}
	checkMessage(t, <-got)
}

func TestSendOTPWithLoginAuth(t *testing.T) { // Office 365 / Outlook advertises AUTH LOGIN XOAUTH2 and rejects AUTH PLAIN
	cert, pool := selfSignedCert(t)
	port, got := fakeSMTPWithAuth(t, cert, false, true, "LOGIN XOAUTH2")
	if err := newTestService(port, false, pool).SendOTP("user@example.com", "123456", "verify_email"); err != nil {
		t.Fatal(err)
	}
	checkMessage(t, <-got)
}

func TestSendOTPWithImplicitTLS(t *testing.T) { // like nodemailer secure:true on 465
	cert, pool := selfSignedCert(t)
	port, got := fakeSMTP(t, cert, true, false)
	if err := newTestService(port, true, pool).SendOTP("user@example.com", "123456", "verify_email"); err != nil {
		t.Fatal(err)
	}
	checkMessage(t, <-got)
}

func TestRefusesPlaintextAuth(t *testing.T) {
	cert, pool := selfSignedCert(t)
	port, _ := fakeSMTP(t, cert, false, false)
	err := newTestService(port, false, pool).SendOTP("user@example.com", "123456", "verify_email")
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected refusal without STARTTLS, got %v", err)
	}
}

func TestHeaderInjectionStripped(t *testing.T) {
	msg := string(newTestService(25, false, nil).buildMessage([]string{"a@example.com\r\nBcc: evil@example.com"}, "Hi\r\nBcc: x@y.z", "body"))
	if strings.Contains(msg, "\r\nBcc:") {
		t.Fatalf("header injection not stripped:\n%s", msg)
	}
}
