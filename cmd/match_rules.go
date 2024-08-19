package cmd

import (
	"Keydd/consts"
	"Keydd/engine_db"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"strings"
)

// MatchRules函数用于匹配规则
func MatchRules(body string, f *proxy.Flow) error {
	for key, pattern := range consts.LodaRules {
		allMatches := pattern.FindAllStringSubmatch(body, -1)
		if len(allMatches) > 0 {
			for _, match := range allMatches {
				//去掉为空 没内容的结果
				if len(match[0]) != 0 {
					matchStr := strings.Trim(match[0], "[]") // 去除中括号
					data := &consts.Keyinfo{
						RuleName:     key,
						Host:         f.Request.URL.Host,
						Req_Path:     f.Request.URL.Path,
						Req_Body:     f.Request.Body,
						Res_Body:     f.Response.Body,
						Key_text:     matchStr,
						Content_Type: f.Response.Header.Get("Content-Type"),
					}
					err := engine_db.WriteDataToDatabase(data)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
