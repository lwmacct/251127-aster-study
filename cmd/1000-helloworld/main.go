package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/astercloud/aster/pkg/agent"
	"github.com/astercloud/aster/pkg/provider"
	"github.com/astercloud/aster/pkg/sandbox"
	"github.com/astercloud/aster/pkg/store"
	"github.com/astercloud/aster/pkg/tools"
	"github.com/astercloud/aster/pkg/tools/builtin"
	"github.com/astercloud/aster/pkg/types"
	"github.com/lwmacct/251125-go-mod-logger/pkg/logger"
)

func main() {
	// 初始化日志系统（从环境变量读取配置）
	if err := logger.InitFromEnv(); err != nil {
		slog.Warn("初始化日志系统失败，使用默认配置", "error", err)
	}
	slog.Info("日志系统初始化成功")

	// 检查 API Key (使用 OpenRouter)
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		slog.Error("OPENROUTER_API_KEY environment variable is required")
		os.Exit(1)
	}

	// 可选：自定义 BaseURL
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1" // 默认 OpenRouter API
	}

	// 创建上下文
	ctx := context.Background()

	// 1. 创建工具注册表并注册内置工具
	toolRegistry := tools.NewRegistry()
	builtin.RegisterAll(toolRegistry)

	// 2. 创建 Sandbox 工厂
	sandboxFactory := sandbox.NewFactory()

	// 3. 创建 Provider 工厂 (使用 OpenRouter)
	providerFactory := &provider.OpenRouterFactory{}

	// 4. 创建 Store
	storePath := ".aster"
	jsonStore, err := store.NewJSONStore(storePath)
	if err != nil {
		slog.Error("Failed to create store", "error", err)
		os.Exit(1)
	}

	// 5. 创建模板注册表
	templateRegistry := agent.NewTemplateRegistry()

	// 注册一个简单的助手模板 (使用 OpenRouter 模型格式)
	templateRegistry.Register(&types.AgentTemplateDefinition{
		ID:           "simple-assistant",
		Model:        "anthropic/claude-sonnet-4.5", // OpenRouter Claude Sonnet
		SystemPrompt: "You are a helpful assistant that can read and write files. When users ask you to read or write files, use the available tools.",
		Tools:        []interface{}{"Read", "Write", "Bash"},
	})

	// 6. 创建依赖
	deps := &agent.Dependencies{
		Store:            jsonStore,
		SandboxFactory:   sandboxFactory,
		ToolRegistry:     toolRegistry,
		ProviderFactory:  providerFactory,
		TemplateRegistry: templateRegistry,
	}

	// 7. 创建 Agent 配置 (使用 OpenRouter)
	config := &types.AgentConfig{
		TemplateID: "simple-assistant",
		ModelConfig: &types.ModelConfig{
			Provider:      "openrouter",
			Model:         "anthropic/claude-sonnet-4.5", // OpenRouter Claude Sonnet
			APIKey:        apiKey,
			BaseURL:       baseURL,
			ExecutionMode: types.ExecutionModeNonStreaming, // 使用非流式模式测试
		},
		Sandbox: &types.SandboxConfig{
			Kind:    types.SandboxKindLocal,
			WorkDir: "./workspace",
		},
	}

	// 8. 创建 Agent
	ag, err := agent.Create(ctx, config, deps)
	if err != nil {
		slog.Error("Failed to create agent", "error", err)
		os.Exit(1)
	}
	defer func() { _ = ag.Close() }()

	slog.Info("Agent created", "id", ag.ID())

	// 9. 订阅事件
	eventCh := ag.Subscribe([]types.AgentChannel{
		types.ChannelProgress,
		types.ChannelMonitor,
	}, nil)

	// 启动事件监听
	go func() {
		for envelope := range eventCh {
			if evt, ok := envelope.Event.(types.EventType); ok {
				switch evt.Channel() {
				case types.ChannelProgress:
					handleProgressEvent(envelope.Event)
				case types.ChannelMonitor:
					handleMonitorEvent(envelope.Event)
				}
			}
		}
	}()

	// 10. 发送消息并等待完成
	slog.Info("--- Test 1: Create a test file ---")
	result, err := ag.Chat(ctx, "Please create a file called test.txt with content 'Hello World'")
	if err != nil {
		slog.Error("Chat failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Assistant response", "text", result.Text, "status", result.Status)

	// 等待事件处理完成
	time.Sleep(1 * time.Second)

	slog.Info("--- Test 2: Read the file back ---")
	result, err = ag.Chat(ctx, "Please read the test.txt file")
	if err != nil {
		slog.Error("Chat failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Assistant response", "text", result.Text, "status", result.Status)

	time.Sleep(1 * time.Second)

	slog.Info("--- Test 3: Run a bash command ---")
	result, err = ag.Chat(ctx, "Please run 'ls -la' command")
	if err != nil {
		slog.Error("Chat failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Assistant response", "text", result.Text, "status", result.Status)

	// 输出状态
	status := ag.Status()
	fmt.Printf("\n\nFinal Status:\n")
	fmt.Printf("  Agent ID: %s\n", status.AgentID)
	fmt.Printf("  State: %s\n", status.State)
	fmt.Printf("  Steps: %d\n", status.StepCount)
	fmt.Printf("  Cursor: %d\n", status.Cursor)
}

func handleProgressEvent(event interface{}) {
	switch e := event.(type) {
	case *types.ProgressTextChunkEvent:
		fmt.Print(e.Delta)
	case *types.ProgressTextChunkStartEvent:
		fmt.Print("\n[Assistant] ")
	case *types.ProgressTextChunkEndEvent:
		// 文本块结束
	case *types.ProgressToolStartEvent:
		fmt.Printf("\n[Tool Start] %s (ID: %s)\n", e.Call.Name, e.Call.ID)
	case *types.ProgressToolEndEvent:
		fmt.Printf("[Tool End] %s - State: %s\n", e.Call.Name, e.Call.State)
	case *types.ProgressToolErrorEvent:
		fmt.Printf("[Tool Error] %s - Error: %s\n", e.Call.Name, e.Error)
	case *types.ProgressDoneEvent:
		fmt.Printf("\n[Done] Step %d - Reason: %s\n", e.Step, e.Reason)
	}
}

func handleMonitorEvent(event interface{}) {
	switch e := event.(type) {
	case *types.MonitorStateChangedEvent:
		fmt.Printf("[State Changed] %s\n", e.State)
	case *types.MonitorTokenUsageEvent:
		fmt.Printf("[Token Usage] Input: %d, Output: %d, Total: %d\n",
			e.InputTokens, e.OutputTokens, e.TotalTokens)
	case *types.MonitorErrorEvent:
		fmt.Printf("[Error] [%s] %s: %s\n", e.Severity, e.Phase, e.Message)
	case *types.MonitorBreakpointChangedEvent:
		fmt.Printf("[Breakpoint] %v -> %v\n", e.Previous, e.Current)
	}
}
