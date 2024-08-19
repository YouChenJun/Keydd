package main

import (
	"Keydd/cmd"
	"Keydd/consts"
	logger "Keydd/log"
	"Keydd/notify"
	"fmt"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"regexp"
	"strings"
	"sync"
)

type ChangeHtml struct {
	proxy.BaseAddon
}

func init() {
	logger.Init()
	cmd.Init()
	fmt.Println(consts.Banner)
	logger.Info.Println("启动成功-监听端口为：9080 - 请先安装证书")
	config, err := cmd.ReadYAMLFile()
	//fmt.Printf("webhook", config.Lark_webhook)
	notify.Init(config.Lark_webhook)
	if err != nil {
		logger.Info.Fatal("读取YAML文件失败：", err)
		return
	}
	// 正则载入到规则列表里面
	for _, rule := range config.Rules {
		if !rule.Enabled {
			continue // 如果规则未启用，则跳过
		}
		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			logger.Info.Fatal("正则表达式编译失败,请检测规则是否正确！：", err)
			return
		}
		consts.LodaRules[rule.Id] = regex
	}
}
func (c *ChangeHtml) Response(f *proxy.Flow) {
	// 使用WaitGroup和带缓冲的channel来限制并发
	var wg sync.WaitGroup
	errChan := make(chan error, 200) // 允许200个并发错误
	contentType := f.Response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") &&
		!strings.HasPrefix(contentType, "application/json") &&
		!strings.HasPrefix(contentType, "application/javascript") {
		return
	}
	f.Response.ReplaceToDecodedBody()
	body := f.Response.Body
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cmd.MatchRules(string(body), f)
		if err != nil {
			select {
			case errChan <- err:
				// 错误已发送到channel
			default:
				// channel已满
				logger.Error.Println("Failed to send error to channel:", err)
			}
		}
	}()
	// 等待goroutine完成
	go func() {
		wg.Wait()
		close(errChan)
	}()
}

func main() {
	opts := &proxy.Options{
		Addr:              ":9080",
		StreamLargeBodies: 2048 * 2048 * 5,
		CaRootPath:        "./cert",
	}
	p, err := proxy.NewProxy(opts)
	if err != nil {
		logger.Error.Fatalf("err:", err)
	}
	logger.Info.Println("启动成功！请在运行文件夹内寻找证书文件，并安装证书！")
	p.AddAddon(&ChangeHtml{})
	//关闭web页面
	//p.AddAddon(web.NewWebAddon(":9081"))
	p.Start()
}
