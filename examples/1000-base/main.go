package main

import (
	"log/slog"

	"github.com/lwmacct/251125-go-mod-logger/pkg/logger"
)

func main() {

	// 初始化日志系统（从环境变量读取配置）
	if err := logger.InitFromEnv(); err != nil {
		slog.Warn("初始化日志系统失败，使用默认配置", "error", err)
	}

	slog.Info("日志系统初始化成功")

}
