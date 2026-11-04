package manager

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"PromAI/pkg/config"
)

type Manager struct {
	cfg       *config.Config
	cmd       *exec.Cmd
	mu        sync.RWMutex
	running   bool
	startTime time.Time
	stopCh    chan struct{}
	client    *http.Client
}

func New(cfg *config.Config) *Manager {
	m := &Manager{
		cfg:    cfg,
		stopCh: make(chan struct{}),
		client: &http.Client{Timeout: 3 * time.Second},
	}
	return m
}

func (m *Manager) IsEnabled() bool {
	if m.cfg == nil {
		return false
	}
	return m.cfg.AI.Agent.Enabled && m.cfg.AI.Agent.AutoStart
}

func (m *Manager) buildEnv() []string {
	env := os.Environ()
	ac := m.cfg.AI.Agent
	if ac.Username != "" && ac.Password != "" {
		env = append(env, "OPENCODE_SERVER_USERNAME="+ac.Username)
		env = append(env, "OPENCODE_SERVER_PASSWORD="+ac.Password)
	} else {
		env = append(env, "OPENCODE_SERVER_USERNAME=opencode")
	}
	for _, kv := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY",
		"DEEPSEEK_API_KEY", "DASHSCOPE_API_KEY", "ZHIPUAI_API_KEY", "MOONSHOT_API_KEY",
		"CEREBRAS_API_KEY", "GROQ_API_KEY", "XAI_API_KEY", "AZURE_OPENAI_API_KEY",
	} {
		if v := os.Getenv(kv); v != "" {
			env = append(env, kv+"="+v)
		}
	}
	return env
}

func (m *Manager) Start(ctx context.Context) error {
	if !m.IsEnabled() {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	ac := m.cfg.AI.Agent
	bin := ac.BinaryPath
	if bin == "" {
		bin = "opencode"
	}
	addr := ac.ListenAddr
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := ac.ListenPort
	if port == 0 {
		port = 4096
	}
	args := []string{"serve", "--hostname", addr, "--port", strconv.Itoa(port)}
	if len(ac.ExtraArgs) > 0 {
		args = append(args, ac.ExtraArgs...)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = m.buildEnv()
	cmd.Dir = "."
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start opencode: %w", err)
	}
	m.cmd = cmd
	m.running = true
	m.startTime = time.Now()
	go m.logStream("stdout", stdout)
	go m.logStream("stderr", stderr)
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()
	go m.healthLoop()
	return nil
}

func (m *Manager) logStream(name string, r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fmt.Printf("[opencode:%s] %s\n", name, sc.Text())
	}
}

func (m *Manager) healthLoop() {
	interval := m.cfg.AI.Agent.HealthCheckInterval
	if interval <= 0 {
		interval = 30
	}
	t := time.NewTicker(time.Duration(interval) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-t.C:
			_ = m.Health()
		}
	}
}

func (m *Manager) baseURL() string {
	ac := m.cfg.AI.Agent
	addr := ac.ListenAddr
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := ac.ListenPort
	if port == 0 {
		port = 4096
	}
	return fmt.Sprintf("http://%s:%d", addr, port)
}

func (m *Manager) doReq(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, m.baseURL()+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	ac := m.cfg.AI.Agent
	if ac.Username != "" && ac.Password != "" {
		req.SetBasicAuth(ac.Username, ac.Password)
	} else {
		req.SetBasicAuth("opencode", "")
	}
	return m.client.Do(req)
}

func (m *Manager) Health() bool {
	resp, err := m.doReq(http.MethodGet, "/global/health", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	if v, ok := out["healthy"].(bool); ok {
		return v
	}
	return resp.StatusCode == 200
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	close(m.stopCh)
	select {
	case <-time.After(2 * time.Second):
		_ = m.cmd.Process.Kill()
	default:
		_ = m.cmd.Process.Kill()
	}
	m.running = false
	return nil
}

func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ac := m.cfg.AI.Agent
	status := map[string]interface{}{
		"enabled": m.IsEnabled(),
		"running": m.running,
		"baseUrl": m.baseURL(),
		"hostname": ac.ListenAddr,
		"port":     ac.ListenPort,
	}
	if m.running {
		status["uptimeSec"] = int(time.Since(m.startTime).Seconds())
	}
	status["healthy"] = m.Health()
	return status
}
