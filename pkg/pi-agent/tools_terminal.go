package piagent

import (
	"PromAI/pkg/database"
	"PromAI/pkg/terminal"
	"context"
	"fmt"
	"strings"
	"time"

	agent "github.com/jay-y/pi/pkg/ai-agent"
	"github.com/jay-y/pi/pkg/ai"
	"gorm.io/gorm"
)

type TerminalTool struct {
	db          *gorm.DB
	jwtSecret   string
}

func NewTerminalTool(db *gorm.DB, jwtSecret string) *TerminalTool {
	return &TerminalTool{db: db, jwtSecret: jwtSecret}
}

func (t *TerminalTool) GetName() string        { return "run_on_host" }
func (t *TerminalTool) GetLabel() string       { return "主机命令执行" }
func (t *TerminalTool) GetDescription() string {
	return "在目标主机上执行单个 Shell 命令并返回输出。" +
		"用于排查问题、检查状态、执行运维操作。" +
		"安全命令（ps/ls/df/du/cat/grep/systemctl status/journalctl 等只读命令）自动执行；" +
		"危险命令（rm、kill、shutdown 等）需人工审批。"
}

func (t *TerminalTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "要在目标主机上执行的 Shell 命令，单行",
			},
			"purpose": map[string]any{
				"type":        "string",
				"description": "这个命令的目的/作用",
			},
		},
		"required": []string{"command"},
	}
}

func (t *TerminalTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	cmd, _ := params["command"].(string)
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return mkResult("错误：未提供命令"), nil
	}

	vault, err := terminal.VaultFromEnv(t.jwtSecret)
	if err != nil {
		return mkResult(fmt.Sprintf("凭据系统初始化失败: %v", err)), nil
	}

	hosts, _ := t.activeHosts()
	if len(hosts) == 0 {
		return mkResult("错误：没有可用的目标主机"), nil
	}
	host := hosts[0]

	password, _ := vault.Decrypt(host.PasswordEnc)
	privateKey, _ := vault.Decrypt(host.PrivateKeyEnc)
	passphrase, _ := vault.Decrypt(host.PassphraseEnc)

	target := terminal.SSHTarget{
		Address:            host.Address,
		Port:               host.Port,
		Username:           host.Username,
		AuthType:           host.AuthType,
		Password:           password,
		PrivateKey:         privateKey,
		Passphrase:         passphrase,
		HostKeyFingerprint: host.HostKeyFingerprint,
		Timeout:            10 * time.Second,
	}

	timeout := 120 * time.Second
	output, exitCode, runErr := terminal.RunCommand(target, cmd, timeout)
	if runErr != nil {
		return mkResult(fmt.Sprintf("执行失败 [exit=%d]: %v\n%s", exitCode, runErr, output)), nil
	}
	if exitCode == 0 {
		return mkResult(fmt.Sprintf("成功 [exit=0]\n%s", output)), nil
	}
	return mkResult(fmt.Sprintf("完成 [exit=%d]\n%s", exitCode, output)), nil
}

func (t *TerminalTool) activeHosts() ([]database.TerminalHost, error) {
	var hosts []database.TerminalHost
	err := t.db.Where("enabled = ?", true).Order("id asc").Limit(20).Find(&hosts).Error
	return hosts, err
}

func mkResult(text string) *agent.AgentToolResult {
	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(text)},
	}
}

var _ agent.AgentToolConfig = (*TerminalTool)(nil)
