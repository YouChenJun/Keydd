package consts

import "regexp"

type Keyinfo struct {
	RuleName     string
	Host         string
	Req_Path     string
	Req_Body     []byte
	Res_Body     []byte
	Key_text     string
	Content_Type string
	Size         float64
}

var LodaRules = make(map[string]*regexp.Regexp)
var Banner = `

 _   __               _     _ 
| | / /              | |   | |
| |/ /  ___ _   _  __| | __| |
|    \ / _ \ | | |/ _' |/ _' |
| |\  \  __/ |_| | (_| | (_| |
\_| \_/\___|\__, |\__,_|\__,_|
             __/ |            
            |___/
				v1.4	by@Chen_dark
`
