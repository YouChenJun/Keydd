package notify

import (
	"Keydd/consts"
	logger "Keydd/log"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var webhookUrl string

func Init(webhook string) {
	webhookUrl = webhook
	fmt.Println("webhook", webhookUrl)
}
func sendMsg(cardtext string) {
	// json
	contentType := "application/json"
	//这里是需要构造发送的数据
	data := `{
		"msg_type": "interactive",
		"card": %s
	}
	`
	sendData := fmt.Sprintf(data, cardtext)
	// request
	if webhookUrl == "" {
		return
	}
	result, err := http.Post(webhookUrl, contentType, strings.NewReader(sendData))
	if err != nil {
		fmt.Printf("post failed, err:%v\n", err)
		return
	}
	defer result.Body.Close()
	rspBody, err := ioutil.ReadAll(result.Body)
	if err != nil {
		logger.Error.Fatalf("ReadAll failed, reqBody: %s, err: %v\n", rspBody, err)
		return
	}
	time.Sleep(1 * time.Second)
}

func TaskBeginSendmsg(info *consts.Keyinfo) {
	cardtext := `
{
    "config": {},
    "i18n_elements": {
        "zh_cn": [
            {
                "tag": "markdown",
                "content": "类型: %s \n站点信息: %s \n匹配到的文本: %s \nuri: %s \n点击我去访问！！: <a href='%s'> \n<at id=all></at>",
                "text_align": "left",
                "text_size": "normal"
            }
        ]
    },
    "i18n_header": {
        "zh_cn": {
            "title": {
                "tag": "plain_text",
                "content": "检测到敏感信息了！"
            },
            "subtitle": {
                "tag": "plain_text",
                "content": ""
            },
            "template": "green",
            "ud_icon": {
                "tag": "standard_icon",
                "token": "safe-vc_outlined"
            }
        }
    }
}
`
	url := fmt.Sprint(info.Host, info.Req_Path)
	data := fmt.Sprintf(cardtext, info.RuleName, info.Host, info.Key_text, info.Req_Path, url)
	sendMsg(data)
}
