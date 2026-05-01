package engine_db

import (
	"Keydd/consts"
	"database/sql"
	"os"
	"testing"
)

// TestInitDB 测试数据库初始化
func TestInitDB(t *testing.T) {
	// 清理测试数据库
	os.Remove("test_data.db")
	defer os.Remove("test_data.db")

	// 初始化测试数据库
	testDB, err := sql.Open("sqlite", "file:test_data.db?cache=shared&mode=rwc")
	if err != nil {
		t.Fatalf("数据库连接失败: %v", err)
	}
	defer testDB.Close()

	// 创建表
	_, err = testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
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
		t.Fatalf("创建表失败: %v", err)
	}

	// 验证表已创建
	var tableName string
	err = testDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='key_info'").Scan(&tableName)
	if err != nil || tableName != "key_info" {
		t.Errorf("表创建失败或不存在")
	}
}

// TestInsertData_NewRecord 测试插入新记录
func TestInsertData_NewRecord(t *testing.T) {
	os.Remove("test_data_insert.db")
	defer os.Remove("test_data_insert.db")

	testDB, _ := sql.Open("sqlite", "file:test_data_insert.db?cache=shared&mode=rwc")
	defer testDB.Close()

	// 创建表
	testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)

	// 准备测试数据
	data := &consts.Keyinfo{
		RuleName:     "test_rule",
		Host:         "example.com",
		Req_Path:     "/api/test",
		Req_Body:     []byte(`{"user":"test"}`),
		Res_Body:     []byte(`{"status":"ok"}`),
		Key_text:     "secret_key_12345",
		Content_Type: "application/json",
	}

	// 测试插入新记录应该返回 true
	result := InsertData(testDB, data)
	if !result {
		t.Errorf("插入新记录失败，期望返回 true，实际返回 false")
	}

	// 验证数据已插入
	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM key_info").Scan(&count)
	if err != nil || count != 1 {
		t.Errorf("插入数据失败，记录数不正确，期望 1，实际 %d", count)
	}
}

// TestInsertData_DuplicateRecord 测试插入重复记录
func TestInsertData_DuplicateRecord(t *testing.T) {
	os.Remove("test_data_dup.db")
	defer os.Remove("test_data_dup.db")

	testDB, _ := sql.Open("sqlite", "file:test_data_dup.db?cache=shared&mode=rwc")
	defer testDB.Close()

	// 创建表
	testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)

	// 准备测试数据
	data := &consts.Keyinfo{
		RuleName:     "jwt_rule",
		Host:         "api.example.com",
		Req_Path:     "/auth/login",
		Req_Body:     []byte(`{"pass":"123"}`),
		Res_Body:     []byte(`{"token":"jwt"}`),
		Key_text:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		Content_Type: "application/json",
	}

	// 第一次插入应该成功
	result1 := InsertData(testDB, data)
	if !result1 {
		t.Fatalf("第一次插入失败")
	}

	// 第二次插入相同数据应该返回 false（因为是重复）
	result2 := InsertData(testDB, data)
	if result2 {
		t.Errorf("重复插入检测失败，期望返回 false，实际返回 true")
	}

	// 验证只有一条记录
	var count int
	testDB.QueryRow("SELECT COUNT(*) FROM key_info").Scan(&count)
	if count != 1 {
		t.Errorf("重复检测失败，记录数不正确，期望 1，实际 %d", count)
	}
}

// TestInsertData_MultipleDistinctRecords 测试插入多个不同的记录
func TestInsertData_MultipleDistinctRecords(t *testing.T) {
	os.Remove("test_data_multi.db")
	defer os.Remove("test_data_multi.db")

	testDB, _ := sql.Open("sqlite", "file:test_data_multi.db?cache=shared&mode=rwc")
	defer testDB.Close()

	testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)

	testCases := []struct {
		name   string
		data   *consts.Keyinfo
		expect bool
	}{
		{
			name: "AWS_KEY",
			data: &consts.Keyinfo{
				RuleName: "aws_key", Host: "s3.amazonaws.com", Req_Path: "/bucket",
				Req_Body: []byte(""), Res_Body: []byte(""), Key_text: "AKIA2Z7K8Q9R5X2Y", Content_Type: "text/plain",
			},
			expect: true,
		},
		{
			name: "API_TOKEN",
			data: &consts.Keyinfo{
				RuleName: "api_token", Host: "api.github.com", Req_Path: "/repos",
				Req_Body: []byte(""), Res_Body: []byte(""), Key_text: "ghp_16C7e42F292c6912E7710c838347Ae178B4a", Content_Type: "application/json",
			},
			expect: true,
		},
		{
			name: "DATABASE_PASS",
			data: &consts.Keyinfo{
				RuleName: "db_password", Host: "db.internal", Req_Path: "/query",
				Req_Body: []byte(""), Res_Body: []byte(""), Key_text: "mysql://admin:P@ssw0rd123@localhost:3306", Content_Type: "text/plain",
			},
			expect: true,
		},
	}

	successCount := 0
	for _, tc := range testCases {
		result := InsertData(testDB, tc.data)
		if result != tc.expect {
			t.Errorf("[%s] 插入失败，期望 %v，实际 %v", tc.name, tc.expect, result)
		}
		if result {
			successCount++
		}
	}

	// 验证所有记录都被插入
	var count int
	testDB.QueryRow("SELECT COUNT(*) FROM key_info").Scan(&count)
	if count != successCount {
		t.Errorf("插入记录数不正确，期望 %d，实际 %d", successCount, count)
	}
}

// TestInsertData_EdgeCases 测试边界情况
func TestInsertData_EdgeCases(t *testing.T) {
	os.Remove("test_data_edge.db")
	defer os.Remove("test_data_edge.db")

	testDB, _ := sql.Open("sqlite", "file:test_data_edge.db?cache=shared&mode=rwc")
	defer testDB.Close()

	testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)

	testCases := []struct {
		name   string
		data   *consts.Keyinfo
		expect bool
	}{
		{
			name: "空字符串",
			data: &consts.Keyinfo{
				RuleName: "", Host: "", Req_Path: "",
				Req_Body: []byte(""), Res_Body: []byte(""), Key_text: "", Content_Type: "",
			},
			expect: true,
		},
		{
			name: "特殊字符",
			data: &consts.Keyinfo{
				RuleName: "rule'with\"quote", Host: "example.com",
				Req_Path: "/api/test?param=value&key=123", Req_Body: []byte("{}"),
				Res_Body: []byte("[]"), Key_text: "key with 'quote\" and <special>", Content_Type: "text/html",
			},
			expect: true,
		},
		{
			name: "长字符串",
			data: &consts.Keyinfo{
				RuleName: "long_rule",
				Host:     "very-long-domain-name-example.example.com",
				Req_Path: "/very/long/path/to/some/api/endpoint/with/many/segments",
				Req_Body: []byte("Lorem ipsum dolor sit amet consectetur adipiscing elit"),
				Res_Body: []byte("Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua"),
				Key_text: "VeryLongKeyStringWithManyCharactersIncludingSymbols!@#$%^&*()",
				Content_Type: "application/json; charset=utf-8",
			},
			expect: true,
		},
		{
			name: "Unicode 字符",
			data: &consts.Keyinfo{
				RuleName: "unicode_rule", Host: "中文.com", Req_Path: "/路径/测试",
				Req_Body: []byte("中文请求"), Res_Body: []byte("中文响应"),
				Key_text: "密钥信息🔐", Content_Type: "application/json",
			},
			expect: true,
		},
	}

	for _, tc := range testCases {
		result := InsertData(testDB, tc.data)
		if result != tc.expect {
			t.Errorf("[%s] 插入失败，期望 %v，实际 %v", tc.name, tc.expect, result)
		}
	}
}

// TestInsertData_DifferentPaths 测试相同Host但不同Path
func TestInsertData_DifferentPaths(t *testing.T) {
	os.Remove("test_data_paths.db")
	defer os.Remove("test_data_paths.db")

	testDB, _ := sql.Open("sqlite", "file:test_data_paths.db?cache=shared&mode=rwc")
	defer testDB.Close()

	testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)

	host := "api.example.com"
	key := "sk_live_abc123def456"

	// 相同 host 和 key，但不同 path
	paths := []string{"/api/users", "/api/orders", "/api/products"}

	for _, path := range paths {
		data := &consts.Keyinfo{
			RuleName: "stripe_key", Host: host, Req_Path: path,
			Req_Body: []byte(""), Res_Body: []byte(""), Key_text: key, Content_Type: "application/json",
		}
		result := InsertData(testDB, data)
		if !result {
			t.Errorf("插入路径 %s 失败", path)
		}
	}

	// 验证插入了所有不同的路径
	var count int
	testDB.QueryRow("SELECT COUNT(*) FROM key_info WHERE Host = ? AND Key_text = ?", host, key).Scan(&count)
	if count != len(paths) {
		t.Errorf("不同路径插入失败，期望 %d，实际 %d", len(paths), count)
	}
}

// TestInsertData_SamePathDifferentRules 测试相同Path但不同Rule
func TestInsertData_SamePathDifferentRules(t *testing.T) {
	os.Remove("test_data_rules.db")
	defer os.Remove("test_data_rules.db")

	testDB, _ := sql.Open("sqlite", "file:test_data_rules.db?cache=shared&mode=rwc")
	defer testDB.Close()

	testDB.Exec(`CREATE TABLE IF NOT EXISTS key_info (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        RuleName VARCHAR(255),
    	Host VARCHAR(255),
    	Req_Path VARCHAR(255),
    	Req_Body TEXT,
    	Res_Body TEXT,
    	Key_text TEXT,    
    	Content_Type VARCHAR(255)
    )`)

	host := "api.example.com"
	path := "/api/config"

	// 相同 host 和 path，但不同 rule 和 key
	rules := []struct {
		rule string
		key  string
	}{
		{"jwt_token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		{"api_key", "sk_live_abc123def456"},
		{"password", "P@ssw0rd123"},
	}

	for _, r := range rules {
		data := &consts.Keyinfo{
			RuleName: r.rule, Host: host, Req_Path: path,
			Req_Body: []byte(""), Res_Body: []byte(""), Key_text: r.key, Content_Type: "application/json",
		}
		result := InsertData(testDB, data)
		if !result {
			t.Errorf("插入规则 %s 失败", r.rule)
		}
	}

	// 验证插入了所有不同的规则
	var count int
	testDB.QueryRow("SELECT COUNT(*) FROM key_info WHERE Host = ? AND Req_Path = ?", host, path).Scan(&count)
	if count != len(rules) {
		t.Errorf("不同规则插入失败，期望 %d，实际 %d", len(rules), count)
	}
}
