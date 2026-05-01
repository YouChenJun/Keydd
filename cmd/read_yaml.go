package cmd

import (
	"Keydd/ai/config"
	"Keydd/consts"
	logger "Keydd/log"
	"Keydd/utils"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

func Init() {
	if !utils.FileExists("./config/rule.yaml") {
		createRuleConfigFile()
	}
	// 创建默认 config.yaml 如果不存在
	if !utils.FileExists("./config/config.yaml") {
		createAIConfigFile()
	}
}

// 读取规则配置文件（rule.yaml）
// 仅包含规则和飞书 webhook
func ReadRuleYAML() (*consts.Rules, error) {
	data, err := ioutil.ReadFile("./config/rule.yaml")
	if err != nil {
		return nil, err
	}
	var config consts.Rules
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// 读取全局配置文件（config.yaml）
// 包含 AI 功能配置
func ReadConfigYAML() (*config.AIConfig, error) {
	if !utils.FileExists("./config/config.yaml") {
		// 如果配置文件不存在，返回默认配置
		return config.DefaultAIConfig(), nil
	}
	data, err := ioutil.ReadFile("./config/config.yaml")
	if err != nil {
		return nil, err
	}
	var cfg struct {
		AI config.AIConfig `yaml:"ai"`
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg.AI, nil
}

// 若没有规则配置文件，则自动创建 rule.yaml
func createRuleConfigFile() {
	configContent := []byte(`# 此处规则配置文件来自 wih，可以自主编写规则
rules:
  # 域名，内置规则
  - id: domain
    enabled: false
  - id: path
    enabled: false
  - id: domain_url
    enabled: false
  - id: ip
    enabled: false
#	pattern: \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}
  # URL主机部分为IP，内置规则
  - id: ip_url
    enabled: false
  # 邮箱 - 误报量大默认关闭
  - id: email
    enabled: false
    pattern: \b[A-Za-z0-9._\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,61}\b
    test_cases:
      - "user@example.com"
  # 二代身份证
  - id: id_card
    enabled: true
    pattern: \b([1-9]\d{5}(19|20)\d{2}((0[1-9])|(1[0-2]))(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx])\b
    test_cases:
      - "110101199001011234"
  # 手机号
  - id: phone
    enabled: true
    pattern: \b1[3-9]\d{9}\b
    test_cases:
      - "13812345678"
  # jwt token (不要修改ID)
  - id: jwt_token
    enabled: true
    pattern: eyJ[A-Za-z0-9_/+\-]{10,}={0,2}\.[A-Za-z0-9_/+\-\\]{15,}={0,2}\.[A-Za-z0-9_/+\-\\]{10,}={0,2}
    test_cases:
      - "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
  # 阿里云 AccessKey ID (不要修改ID)
  - id: Aliyun_AK_ID
    enabled: true
    pattern: \bLTAI[A-Za-z\d]{12,30}\b
    test_cases:
      - "LTAI5tFakeAliyunKeyExample123"
  # 腾讯云 AccessKey ID (不要修改ID)
  - id: QCloud_AK_ID
    enabled: true
    pattern: \bAKID[A-Za-z\d]{13,40}\b
    test_cases:
      - "AKIDFakeQCloudKeyExample123456"
  # 京东云 AccessKey ID (不要修改ID)
  - id: JDCloud_AK_ID
    enabled: true
    pattern: \bJDC_[0-9A-Z]{25,40}\b
    test_cases:
      - "JDC_FAKEKEY1234567890ABCDEFGHIJK"
  # 亚马逊 AccessKey ID
  - id: AWS_AK_ID
    enabled: true
    pattern: '["''](?:A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}["'']'
    test_cases:
      - '"AKIAIOSFODNN7EXAMPLE"'
  # 火山引擎 AccessKey ID
  - id: VolcanoEngine_AK_ID
    enabled: true
    pattern: \b(?:AKLT|AKTP)[a-zA-Z0-9]{35,50}\b
    test_cases:
      - "AKLTfakeVolcanoEngineKey1234567890abcdef12345"
  # 金山云 AccessKey ID
  - id: Kingsoft_AK_ID
    enabled: true
    pattern: \bAKLT[a-zA-z0-9-_]{16,28}\b
    test_cases:
      - "AKLTfakeKingsoftKey1234567"
  # 谷歌云 AccessKey ID
  - id: GCP_AK_ID
    enabled: true
    pattern: \bAIza[0-9A-Za-z_\-]{35}\b
    test_cases:
      - "AIzaSyFakeGoogleAPIKeyABCDEFGHIJK12345"
  # 提取 SecretKey, 内置规则
  - id: secret_key
    enabled: true
  # Bearer Token
  - id: bearer_token
    enabled: true
    pattern: \b[Bb]earer\s+[a-zA-Z0-9\-=._+/\\]{20,500}\b
    test_cases:
      - "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9abcdefghij"
  # Basic Token
  - id: basic_token
    enabled: true
    pattern: \b[Bb]asic\s+[A-Za-z0-9+/]{18,}={0,2}\b
    test_cases:
      - "Basic dXNlcm5hbWU6cGFzc3dvcmQ="
  # Auth Token
  - id: auth_token
    enabled: true
    pattern: '["''\[]*[Aa]uthorization["''\]]*\s*[:=]\s*[''"]?\b(?:[Tt]oken\s+)?[a-zA-Z0-9\-_+/]{20,500}[''"]?'
    test_cases:
      - '"Authorization": "Token abcdefghijklmnopqrstuvwxyz1234"'
  # PRIVATE KEY
  - id: private_key
    enabled: true
    pattern: -----\s*?BEGIN[ A-Z0-9_-]*?PRIVATE KEY\s*?-----[a-zA-Z0-9\/\n\r=+]*-----\s*?END[ A-Z0-9_-]*? PRIVATE KEY\s*?-----
    test_cases:
      - "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xHn\n-----END RSA PRIVATE KEY-----"
  #Gitlab V2 Token
  - id: gitlab_v2_token
    enabled: true
    pattern: \b(glpat-[a-zA-z0-9\-=_]{20,22})\b
    test_cases:
      - "glpat-FakeGitLabToken12345"
  #Github Token
  - id: github_token
    enabled: true
    pattern: \b((?:ghp|gho|ghu|ghs|ghr|github_pat)_[a-zA-Z0-9_]{36,255})\b
    test_cases:
      - "ghp_FakeGitHubPersonalAccessTokenExample1234567890"
  #腾讯云 API网关 APPKEY
  - id: qcloud_api_gateway_appkey
    enabled: true
    pattern: \bAPID[a-zA-Z0-9]{32,42}\b
    test_cases:
      - "APIDFakeQCloudAPIGatewayAppKey12345678901234567"
  #微信 公众号/小程序 APPID
  - id: wechat_appid
    enabled: true
    pattern: '["''](wx[a-z0-9]{15,18})["'']'
    test_cases:
      - '"wx1234567890abcdef0"'
  #企业微信 corpid
  - id: wechat_corpid
    enabled: true
    pattern: '["''](ww[a-z0-9]{15,18})["'']'
    test_cases:
      - '"ww1234567890abcdef0"'
  #微信公众号
  - id: wechat_id
    enabled: true
    pattern: '["''](gh_[a-z0-9]{11,13})["'']'
    test_cases:
      - '"gh_1234567890a"'
  # 密码
  - id: password
    enabled: true
    pattern: (?i)(?:admin_?pass|password|[a-z]{3,15}_?password|user_?pass|user_?pwd|admin_?pwd)\\?['"]*\s*[:=]\s*\\?['"][a-z0-9!@#$%&*]{5,50}\\?['"]
    test_cases:
      - "password='mysecret123'"
  # 企业微信 webhook
  - id: wechat_webhookurl
    enabled: true
    pattern: \bhttps://qyapi.weixin.qq.com/cgi-bin/webhook/send\?key=[a-zA-Z0-9\-]{25,50}\b
    test_cases:
      - "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abcdef12-3456-7890-abcd-ef1234567890"
  # 钉钉 webhook
  - id: dingtalk_webhookurl
    enabled: true
    pattern: \bhttps://oapi.dingtalk.com/robot/send\?access_token=[a-z0-9]{50,80}\b
    test_cases:
      - "https://oapi.dingtalk.com/robot/send?access_token=abcdef1234567890abcdef1234567890abcdef1234567890ab"
  # 飞书 webhook
  - id: feishu_webhookurl
    enabled: true
    pattern: \bhttps://open.feishu.cn/open-apis/bot/v2/hook/[a-z0-9\-]{25,50}\b
    test_cases:
      - "https://open.feishu.cn/open-apis/bot/v2/hook/abcdef12-3456-7890-abcd-ef"
  # slack webhook
  - id: slack_webhookurl
    enabled: true
    pattern: \bhttps://hooks.slack.com/services/[a-zA-Z0-9\-_]{6,12}/[a-zA-Z0-9\-_]{6,12}/[a-zA-Z0-9\-_]{15,24}\b
    test_cases:
      - "https://hooks.slack.com/services/ABCDEF12/GHIJKL34/MNOPQRSTUVWXYZabc"
  # grafana api key
  - id: grafana_api_key
    enabled: true
    pattern: \beyJrIjoi[a-zA-Z0-9\-_+/]{50,100}={0,2}\b
    test_cases:
      - "eyJrIjoiABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789AB"
  # grafana cloud api token
  - id: grafana_cloud_api_token
    enabled: true
    pattern: \bglc_[A-Za-z0-9\-_+/]{32,200}={0,2}\b
    test_cases:
      - "glc_FakeGrafanaCloudAPITokenABCDEFGHIJKLMNOPQRSTU"
  # grafana service account token
  - id: grafana_service_account_token
    enabled: true
    pattern: \bglsa_[A-Za-z0-9]{32}_[A-Fa-f0-9]{8}\b
    test_cases:
      - "glsa_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef12_1a2b3c4d"
  - id: app_key
    enabled: true
    pattern: \b(?:VUE|APP|REACT)_[A-Z_0-9]{1,15}_(?:KEY|PASS|PASSWORD|TOKEN|APIKEY)['"]*[:=]"(?:[A-Za-z0-9_\-]{15,50}|[a-z0-9/+]{50,100}==?)"
    test_cases:
      - 'APP_SECRET_KEY:"FakeAppSecretKeyValue1234567"'

# 排除规则， 支持字段 id, content, target , source 逻辑为 and ，如果是正则匹配，需要使用 regex: 开头
exclude_rules:
  # 排除站点 https://cc.163.com 中 类型为 secret_key 的内容
  - name: "不收集 cc.163.com 的 secret_key" # 排除规则名称，无实际意义
    id: secret_key
    target: regex:cc\.163\.com
    enabled: true

  - name: "不收集 open.work.weixin.qq.com 的 bearer_token"
    id: bearer_token
    target: https://open.work.weixin.qq.com
    content: regex:Bearer\s+
    enabled: true

  - name: "过滤来自首页的内容"
    source_tag: page
    enabled: false

# 这里需要去获取飞书webhook
lark_Webhook:
`)
	err := os.MkdirAll("./config", os.ModePerm)
	if err != nil {
		logger.Error.Fatal("创建目录失败:", err)
		return
	}
	err = os.WriteFile("./config/rule.yaml", configContent, 0644)
	if err != nil {
		logger.Error.Fatal("创建规则配置文件失败", err)
	}
	logger.Info.Fatal("规则配置文件创建成功，需要手动修改配置文件中的连接信息")
}

// 创建默认的全局配置文件 config.yaml（包含 AI 配置）
func createAIConfigFile() {
	configContent := []byte(`# Keydd 全局配置
# 此文件存放 AI 功能配置，规则配置请查看 rule.yaml

# AI 分析功能配置（基于 trpc-agent-go + LLM 的自动化 API 分析）
ai:
  # 是否启用 AI 分析功能（默认关闭，需要配置好 LLM 后再开启）
  enabled: false

  # 大模型配置（支持 OpenAI 及兼容格式，如 DeepSeek、Ollama 等）
  llm:
    provider: openai              # 提供商: openai / deepseek / ollama
    model: gpt-4o-mini            # 模型名称
    api_key: ""                   # API Key（也可通过 OPENAI_API_KEY 环境变量设置）
    base_url: "https://api.openai.com/v1"  # API 地址（兼容其他 OpenAI 格式接口）
    temperature: 0.3              # 生成温度 (0.0-1.0)
    max_tokens: 4096              # 最大生成 token 数
    timeout: 60                   # 请求超时（秒）

  # 数据库存储配置
  store:
    type: sqlite                    # 数据库类型: sqlite / postgres / mysql
    sqlite_path: "data_ai.db"      # SQLite 文件路径（type=sqlite 时有效）
    postgres_dsn: ""               # PostgreSQL 连接字符串（type=postgres 时有效）
    # 格式: "host=localhost port=5432 user=keydd password=xxx dbname=keydd sslmode=disable"

  # 分析行为配置
  analysis:
    business_analysis_enabled: true       # 业务分析（判断是什么业务系统，接口功能）
    only_analyze_xhr: false               # 只分析 XHR/fetch 请求，默认 false

  # Memory Service 配置（Agent 会话记忆）
  memory:
    backend: inmemory              # 后端类型: redis / inmemory
    redis_addr: ""                 # Redis 地址（backend=redis 时需配置）
    auto_extract: true             # 是否启用自动记忆提取
    check_interval: 5              # 自动提取检查间隔（消息条数）

  # 可观测性配置（Langfuse 链路追踪）
  observability:
    enabled: false          # 是否启用 Langfuse 链路上报（默认关闭）
    public_key: ""          # Langfuse Public Key
    secret_key: ""          # Langfuse Secret Key
    host: "cloud.langfuse.com:443"
    insecure: false         # 是否使用非加密连接（仅本地自建开发时设置 true）
`)
	err := os.MkdirAll("./config", os.ModePerm)
	if err != nil {
		logger.Error.Fatal("创建目录失败:", err)
		return
	}
	err = os.WriteFile("./config/config.yaml", configContent, 0644)
	if err != nil {
		logger.Error.Fatal("创建全局配置文件失败", err)
	}
	logger.Info.Fatal("全局配置文件创建成功，需要手动修改配置文件中的 AI 连接信息")
}
