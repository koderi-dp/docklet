package secrets

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// SSH_ASKPASS mechanism. The app starts `ssh` with SSH_ASKPASS pointing at its
// own executable. When ssh needs a secret it re-invokes that executable with the
// prompt as argv[1]; at the very top of main() RunAskpassIfRequested notices the
// DOCKLET_ASKPASS_* environment, asks the parent over a one-shot loopback
// listener (127.0.0.1, ephemeral port, random 192-bit token; Go unix sockets do not dial on Windows) and prints the answer. The secret
// is never in argv, in a file or in the child's environment.

const (
	envSock  = "DOCKLET_ASKPASS_ADDR"
	envToken = "DOCKLET_ASKPASS_TOKEN"
)

// AskpassServer answers ssh prompts for one ssh child process.
type AskpassServer struct {
	sock     string
	token    string
	ln       net.Listener
	password string
	phrase   string

	mu       sync.Mutex
	answered map[string]bool
	closed   bool
}

// StartAskpass listens on a private socket. Either secret may be empty. Each kind
// is released at most once (a wrong secret is not retried by ssh), and anything
// that is not a password/passphrase prompt (e.g. "trust this host key?") is
// answered with nothing, which ssh treats as a refusal.
func StartAskpass(password, passphrase string) (*AskpassServer, error) {
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &AskpassServer{token: hex.EncodeToString(tok),
		password: password, phrase: passphrase, answered: map[string]bool{}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("askpass socket: %w", err)
	}
	s.ln = ln
	s.sock = ln.Addr().String()
	go s.serve()
	return s, nil
}

// Env returns the environment variables to add to the ssh child.
func (s *AskpassServer) Env() ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return []string{
		"SSH_ASKPASS=" + exe,
		"SSH_ASKPASS_REQUIRE=force",
		"DISPLAY=docklet:0",
		envSock + "=" + s.sock,
		envToken + "=" + s.token,
	}, nil
}

// Close stops the server and removes the socket directory.
func (s *AskpassServer) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	_ = s.ln.Close()
}

func (s *AskpassServer) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *AskpassServer) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return
	}
	token, prompt, _ := strings.Cut(strings.TrimRight(line, "\r\n"), "\t")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		return
	}
	_, _ = c.Write([]byte(s.answer(prompt) + "\n"))
}

// answer picks the secret for a prompt; "" means refuse.
func (s *AskpassServer) answer(prompt string) string {
	p := strings.ToLower(prompt)
	kind, secret := "", ""
	switch {
	case strings.Contains(p, "passphrase"):
		kind, secret = KindSSHPassphrase, s.phrase
	case strings.Contains(p, "password"):
		kind, secret = KindSSHPassword, s.password
	}
	if kind == "" || secret == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.answered[kind] {
		return ""
	}
	s.answered[kind] = true
	return secret
}

// RunAskpassIfRequested must be called first thing in main(), before Wails
// starts. When the process was launched by ssh as its askpass helper it answers
// the prompt on stdout and returns true (the caller must then return/exit).
func RunAskpassIfRequested() bool {
	sock, token := os.Getenv(envSock), os.Getenv(envToken)
	if sock == "" || token == "" {
		return false
	}
	prompt := ""
	if len(os.Args) > 1 {
		prompt = strings.Join(strings.Fields(strings.Join(os.Args[1:], " ")), " ")
	}
	secret, err := askParent(sock, token, prompt)
	if err != nil || secret == "" {
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, secret)
	return true
}

func askParent(sock, token, prompt string) (string, error) {
	c, err := net.DialTimeout("tcp", sock, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := fmt.Fprintf(c, "%s\t%s\n", token, prompt); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
