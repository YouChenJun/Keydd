package main

import (
	"Keydd/ai"
	"Keydd/ai/config"
	"Keydd/cmd"
	"Keydd/consts"
	logger "Keydd/log"
	"Keydd/notify"
	"Keydd/proxy"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	mitm_proxy "github.com/lqqyt2423/go-mitmproxy/proxy"
)

func init() {
	// 初始化日志系统
	logger.Init()

	// 初始化配置文件（如果不存在则自动创建）
	cmd.Init()

	// 打印 Banner
	fmt.Println(consts.Banner)
	logger.Info.Println("启动成功-监听端口为：9080 - 请先安装证书")

	// 读取规则配置文件 (rule.yaml)
	rulesConfig, err := cmd.ReadRuleYAML()
	if err != nil {
		logger.Info.Fatal("读取规则配置文件失败：", err)
	}

	// 读取全局配置文件 (config.yaml)
	_, err = cmd.ReadConfigYAML()
	if err != nil {
		logger.Info.Fatal("读取全局配置文件失败：", err)
	}

	// 初始化飞书 Webhook 通知
	notify.Init(rulesConfig.Lark_webhook)

	// 验证规则配置（警告模式：有问题仅输出警告，不阻止启动）
	if validErrs := cmd.ValidateRules(rulesConfig.Rules); len(validErrs) > 0 {
		cmd.PrintValidationReport(rulesConfig.Rules, validErrs)
	}

	// 编译并加载所有启用的检测规则
	for _, rule := range rulesConfig.Rules {
		if !rule.Enabled {
			continue
		}
		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			logger.Info.Fatal("正则表达式编译失败,请检测规则是否正确！：", err)
		}
		consts.LodaRules[rule.Id] = regex
	}
}

// Application 应用程序主结构
type Application struct {
	rulesConfig *consts.Rules
	aiConfig    *config.AIConfig
	aiSystem    *ai.AISystem
	proxy       *mitm_proxy.Proxy
}

// NewApplication 创建新的应用程序实例
func NewApplication(rulesConfig *consts.Rules, aiConfig *config.AIConfig) *Application {
	return &Application{
		rulesConfig: rulesConfig,
		aiConfig:    aiConfig,
	}
}

// Initialize 初始化应用程序（包括 AI 系统和 MITM 代理）
func (app *Application) Initialize(ctx context.Context) error {
	// 初始化 AI 分析系统（可选）
	if app.aiConfig.Enabled {
		aiSystem, err := ai.InitializeAISystem(ctx, *app.aiConfig)
		if err != nil {
			logger.Error.Printf("初始化 AI 系统失败: %v", err)
			return err
		}
		app.aiSystem = aiSystem
	} else {
		logger.Info.Println("[AI] AI 分析功能未启用，跳过初始化")
	}

	// 检查端口 9080 是否被占用
	if err := checkPortAvailable(":9080"); err != nil {
		return err
	}

	// 初始化 MITM 代理
	opts := &mitm_proxy.Options{
		Addr:              ":9080",
		StreamLargeBodies: 2048 * 2048 * 5,
		CaRootPath:        "./cert",
	}

	var err error
	app.proxy, err = mitm_proxy.NewProxy(opts)
	if err != nil {
		return fmt.Errorf("创建代理失败: %w", err)
	}

	// 创建代理响应处理器
	handler := proxy.NewHandler(app.aiSystem, *app.aiConfig)
	addon := proxy.NewAddon(handler)
	app.proxy.AddAddon(addon)

	return nil
}

// checkPortAvailable 检查端口是否可用，如果被占用则返回错误
func checkPortAvailable(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// 检查是否为端口占用错误（macOS 和 Linux 通用）
		errMsg := err.Error()
		if strings.Contains(errMsg, "address already in use") ||
			strings.Contains(errMsg, "EADDRINUSE") {
			// 根据操作系统显示不同的提示命令
			port := strings.TrimPrefix(addr, ":")
			var hint string
			if runtime.GOOS == "windows" {
				hint = fmt.Sprintf("提示：运行 'netstat -ano | findstr :%s' 查看占用端口的进程，然后使用 'taskkill /PID <进程ID> /F' 关闭进程", port)
			} else {
				hint = fmt.Sprintf("提示：运行 'lsof -i :%s' 查看占用端口的进程，然后使用 'kill <进程ID>' 关闭进程", port)
			}
			return fmt.Errorf("端口 %s 已被占用！\n请先关闭占用该端口的进程，或修改配置文件使用其他端口\n%s", addr, hint)
		}
		return fmt.Errorf("检查端口 %s 可用性失败: %w", addr, err)
	}
	// 立即关闭监听器，因为我们只是检查端口可用性
	listener.Close()
	return nil
}

// Start 启动代理服务
func (app *Application) Start() error {
	logger.Info.Println("启动成功！请在运行文件夹内寻找证书文件，并安装证书！")
	return app.proxy.Start()
}

// Shutdown 优雅关闭应用程序
func (app *Application) Shutdown(ctx context.Context) error {
	// 关闭 AI 系统
	if app.aiSystem != nil {
		if err := app.aiSystem.Shutdown(ctx); err != nil {
			logger.Error.Printf("关闭 AI 系统失败: %v", err)
		}
	}

	logger.Info.Println("应用程序已安全关闭")
	return nil
}

// setupGracefulShutdown 设置优雅关闭处理
func (app *Application) setupGracefulShutdown() <-chan error {
	errChan := make(chan error, 1)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info.Println("收到关闭信号，正在优雅停止...")
		err := app.Shutdown(context.Background())
		errChan <- err
		os.Exit(0)
	}()

	return errChan
}

func main() {
	// 解析命令行参数
	testRules := flag.Bool("test-rules", false, "验证规则文件并退出，不启动代理服务")
	flag.Parse()

	// 如果指定了 --test-rules，则只验证规则
	if *testRules {
		config, err := cmd.ReadRuleYAML()
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取规则文件失败: %v\n", err)
			os.Exit(1)
		}
		errs := cmd.ValidateRules(config.Rules)
		cmd.PrintValidationReport(config.Rules, errs)
		if len(errs) > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 读取规则配置文件 (rule.yaml)
	rulesConfig, err := cmd.ReadRuleYAML()
	if err != nil {
		logger.Error.Fatalf("读取规则配置文件失败: %v", err)
	}

	// 读取全局配置文件 (config.yaml) - 包含 AI 配置
	aiConfig, err := cmd.ReadConfigYAML()
	if err != nil {
		logger.Error.Fatalf("读取全局配置文件失败: %v", err)
	}

	// 创建应用程序实例
	app := NewApplication(rulesConfig, aiConfig)

	// 初始化应用程序
	if err := app.Initialize(context.Background()); err != nil {
		logger.Error.Fatalf("应用程序初始化失败: %v", err)
	}

	// 设置优雅关闭处理
	_ = app.setupGracefulShutdown()

	// 启动代理服务（阻塞直到出错或关闭）
	if err := app.Start(); err != nil {
		logger.Error.Fatalf("代理启动失败: %v", err)
	}
}
