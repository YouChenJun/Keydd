package notify

import (
	consts2 "Keydd/consts"
	"fmt"
	"testing"
)

func TestSendmesg(t *testing.T) {
	fmt.Println("qqq")
	task := &consts2.Keyinfo{
		RuleName:     "微信APPid",
		Host:         "192.168.56.1",
		Req_Path:     "/",
		Key_text:     "https://open.feishu.cn/open-apis/bot/v2/hook/",
		Content_Type: "text/html",
	}
	TaskBeginSendmsg(task)
}
