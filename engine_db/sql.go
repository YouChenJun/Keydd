package engine_db

import (
	"Keydd/consts"
	logger "Keydd/log"
	"Keydd/notify"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

var db *sql.DB
var err error

func init() {
	db = InitDB()
}

// 初始化数据库连接
func InitDB() *sql.DB {
	db, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		logger.Error.Printf("sqlerr:", err)
	}

	// 创建表的语句
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)
	if err != nil {
		log.Printf("sqlerr:", err)
	}
	return db

}
func WriteDataToDatabase(data *consts.Keyinfo) error {
	logger.Info.Println("检测到敏感信息:", data.Key_text)
	if InsertData(db, data) {
		notify.TaskBeginSendmsg(data)
	}
	return nil
}

// 插入数据-存在true -不存在false
func InsertData(db *sql.DB, data *consts.Keyinfo) bool {
	var count int
	query := "SELECT COUNT(*) FROM key_info WHERE Host =? AND Req_Path =? AND RuleName =? AND Key_text=?"
	err = db.QueryRow(query, data.Host, data.Req_Path, data.RuleName, data.Key_text).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		if err != nil {
			logger.Error.Printf("查询失败:", err)
		}
	}
	if count > 0 {
		return false
	} else {
		stmt := "INSERT INTO key_info (RuleName, Host, Req_Path,Req_Body,Res_Body,Key_text,Content_Type) VALUES (?,?,?,?,?,?,?)"
		_, err = db.Exec(stmt, data.RuleName, data.Host, data.Req_Path, string(data.Req_Body), string(data.Res_Body), data.Key_text, data.Content_Type)
		if err != nil {
			logger.Error.Printf("插入失败:", err)
		}
		return true
	}
}
