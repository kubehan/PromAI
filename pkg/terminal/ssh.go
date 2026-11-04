package terminal

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHTarget struct {
	Address            string
	Port               int
	Username           string
	AuthType           string
	Password           string
	PrivateKey         string
	Passphrase         string
	HostKeyFingerprint string
	Timeout            time.Duration
}

func (t SSHTarget) addr() string {
	port := t.Port
	if port <= 0 {
		port = 22
	}
	return net.JoinHostPort(t.Address, strconv.Itoa(port))
}

func (t SSHTarget) authMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	switch strings.ToLower(strings.TrimSpace(t.AuthType)) {
	case "private_key", "key":
		if strings.TrimSpace(t.PrivateKey) == "" {
			return nil, errors.New("未配置 SSH 私钥")
		}
		var (
			signer ssh.Signer
			err    error
		)
		if strings.TrimSpace(t.Passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(t.PrivateKey), []byte(t.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(t.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("解析 SSH 私钥失败: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	default:
		if t.Password == "" {
			return nil, errors.New("未配置 SSH 密码")
		}
		methods = append(methods, ssh.Password(t.Password))
	}
	return methods, nil
}

func hostKeyCallback(expected string) ssh.HostKeyCallback {
	expected = strings.TrimSpace(expected)
	expected = strings.TrimPrefix(expected, "SHA256:")
	if expected == "" {
		// 与常见终端工具保持一致：未配置指纹时接受主机密钥。
		return ssh.InsecureIgnoreHostKey()
	}
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		sum := sha256.Sum256(key.Marshal())
		got := base64.RawStdEncoding.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			return fmt.Errorf("主机密钥指纹不匹配，期望 %s，实际 SHA256:%s", expected, got)
		}
		return nil
	}
}

func Dial(target SSHTarget) (*ssh.Client, error) {
	if strings.TrimSpace(target.Address) == "" {
		return nil, errors.New("主机地址不能为空")
	}
	if strings.TrimSpace(target.Username) == "" {
		return nil, errors.New("SSH 用户名不能为空")
	}
	timeout := target.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	auths, err := target.authMethods()
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            target.Username,
		Auth:            auths,
		HostKeyCallback: hostKeyCallback(target.HostKeyFingerprint),
		Timeout:         timeout,
	}
	conn, err := net.DialTimeout("tcp", target.addr(), timeout)
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %w", target.addr(), err)
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, target.addr(), cfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SSH 握手失败: %w", err)
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

// RunCommand 在主机上执行非交互命令，返回合并后的输出与退出码。
func RunCommand(target SSHTarget, command string, timeout time.Duration) (string, int, error) {
	if strings.TrimSpace(command) == "" {
		return "", -1, errors.New("命令不能为空")
	}
	client, err := Dial(target)
	if err != nil {
		return "", -1, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()

	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	select {
	case err = <-done:
	case <-time.After(timeout):
		_ = session.Close()
		<-done
		return buf.String(), -1, fmt.Errorf("命令执行超时（%s）", timeout)
	}

	exitCode := 0
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitStatus()
			return buf.String(), exitCode, fmt.Errorf("命令退出码 %d", exitCode)
		}
		return buf.String(), -1, err
	}
	return buf.String(), exitCode, nil
}

type ShellSession struct {
	client    *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	reader    *io.PipeReader
	writer    *io.PipeWriter
	closeOnce sync.Once
}

// OpenShell 创建带 PTY 的交互式 Shell，兼容 vim/top/tail -f 等程序。
func OpenShell(target SSHTarget, cols, rows int) (*ShellSession, error) {
	client, err := Dial(target)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 30
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("申请 PTY 失败: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, err
	}
	pr, pw := io.Pipe()
	session.Stdout = pw
	session.Stderr = pw
	if err := session.Shell(); err != nil {
		_ = stdin.Close()
		_ = session.Close()
		_ = client.Close()
		_ = pw.Close()
		return nil, fmt.Errorf("启动 Shell 失败: %w", err)
	}
	shell := &ShellSession{
		client:  client,
		session: session,
		stdin:   stdin,
		reader:  pr,
		writer:  pw,
	}
	go func() {
		_ = session.Wait()
		_ = pw.Close()
	}()
	return shell, nil
}

func (s *ShellSession) Read(p []byte) (int, error) { return s.reader.Read(p) }

func (s *ShellSession) Write(p []byte) (int, error) { return s.stdin.Write(p) }

func (s *ShellSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.session.WindowChange(rows, cols)
}

func (s *ShellSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		_ = s.stdin.Close()
		_ = s.session.Close()
		_ = s.client.Close()
		_ = s.writer.Close()
		err = s.reader.Close()
	})
	return err
}
