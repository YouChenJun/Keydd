package utils

import (
	"Keydd/log"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	// 初始化日志系统以支持所有测试
	log.Init()
}

// TestFileExists_ExistingFile 测试文件存在检查
func TestFileExists_ExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// 创建测试文件
	if err := os.WriteFile(testFile, []byte("test content"), 0666); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 文件应该存在
	if !FileExists(testFile) {
		t.Errorf("FileExists 返回 false，期望 true")
	}
}

// TestFileExists_NonExistingFile 测试不存在的文件
func TestFileExists_NonExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "nonexistent.txt")

	// 文件不应该存在
	if FileExists(testFile) {
		t.Errorf("FileExists 返回 true，期望 false")
	}
}

// TestFileExists_Directory 测试目录
func TestFileExists_Directory(t *testing.T) {
	tempDir := t.TempDir()

	// 目录不应该被视为文件
	if FileExists(tempDir) {
		t.Errorf("FileExists 对目录返回 true，期望 false")
	}
}

// TestReadJSONFile_ValidFile 测试读取有效的 JSON 文件
func TestReadJSONFile_ValidFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.json")
	testData := []byte(`{"name":"test","value":123}`)

	// 创建测试文件
	if err := os.WriteFile(testFile, testData, 0666); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 读取文件
	data, err := ReadJSONFile(testFile)
	if err != nil {
		t.Errorf("ReadJSONFile 失败: %v", err)
	}

	// 验证内容
	if string(data) != string(testData) {
		t.Errorf("读取的内容不匹配，期望 %s，实际 %s", string(testData), string(data))
	}
}

// TestReadJSONFile_NonExistingFile 测试读取不存在的文件
func TestReadJSONFile_NonExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "nonexistent.json")

	// 应该返回错误
	data, err := ReadJSONFile(testFile)
	if err == nil {
		t.Errorf("ReadJSONFile 应该返回错误")
	}
	if data != nil {
		t.Errorf("ReadJSONFile 应该返回 nil 数据")
	}
}

// TestReadJSONFile_EmptyFile 测试读取空文件
func TestReadJSONFile_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "empty.json")

	// 创建空文件
	if err := os.WriteFile(testFile, []byte(""), 0666); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 读取文件
	data, err := ReadJSONFile(testFile)
	if err != nil {
		t.Errorf("ReadJSONFile 失败: %v", err)
	}

	// 验证返回空数据
	if len(data) != 0 {
		t.Errorf("ReadJSONFile 应该返回空数据")
	}
}

// TestDirExists_ExistingDirectory 测试目录存在检查
func TestDirExists_ExistingDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// 目录应该存在
	if !DirExists(tempDir) {
		t.Errorf("DirExists 返回 false，期望 true")
	}
}

// TestDirExists_NonExistingDirectory 测试不存在的目录
func TestDirExists_NonExistingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	nonexistentDir := filepath.Join(tempDir, "nonexistent")

	// 目录不应该存在
	if DirExists(nonexistentDir) {
		t.Errorf("DirExists 返回 true，期望 false")
	}
}

// TestCopyFile_Success 测试文件复制成功
func TestCopyFile_Success(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.txt")
	destFile := filepath.Join(tempDir, "dest.txt")
	testData := []byte("test content for copying")

	// 创建源文件
	if err := os.WriteFile(sourceFile, testData, 0666); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	// 复制文件
	if err := CopyFile(sourceFile, destFile); err != nil {
		t.Errorf("CopyFile 失败: %v", err)
	}

	// 验证目标文件存在
	if !FileExists(destFile) {
		t.Errorf("目标文件不存在")
	}

	// 验证内容相同
	destData, _ := os.ReadFile(destFile)
	if string(destData) != string(testData) {
		t.Errorf("复制的内容不匹配")
	}
}

// TestCopyFile_NonExistingSource 测试复制不存在的源文件
func TestCopyFile_NonExistingSource(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "nonexistent.txt")
	destFile := filepath.Join(tempDir, "dest.txt")

	// 应该返回错误
	if err := CopyFile(sourceFile, destFile); err == nil {
		t.Errorf("CopyFile 应该返回错误")
	}
}

// TestCopyFile_LargeFile 测试复制大文件
func TestCopyFile_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "large_source.txt")
	destFile := filepath.Join(tempDir, "large_dest.txt")

	// 创建大文件（1MB）
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	if err := os.WriteFile(sourceFile, largeData, 0666); err != nil {
		t.Fatalf("创建大文件失败: %v", err)
	}

	// 复制文件
	if err := CopyFile(sourceFile, destFile); err != nil {
		t.Errorf("CopyFile 失败: %v", err)
	}

	// 验证内容相同
	destData, _ := os.ReadFile(destFile)
	if len(destData) != len(largeData) {
		t.Errorf("文件大小不匹配")
	}
}

// TestReadFileData_ValidFile 测试读取文件数据
func TestReadFileData_ValidFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.dat")
	testData := []byte("测试数据内容")

	// 创建测试文件
	if err := os.WriteFile(testFile, testData, 0666); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 读取文件
	data := ReadFileData(testFile)

	// 验证内容
	if string(data) != string(testData) {
		t.Errorf("读取的内容不匹配")
	}
}

// TestReadFileData_NonExistingFile 测试读取不存在的文件
func TestReadFileData_NonExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "nonexistent.dat")

	// 应该返回 nil（会尝试输出日志但不会 crash）
	data := ReadFileData(testFile)
	if data != nil {
		t.Errorf("ReadFileData 应该返回 nil")
	}
}

// TestReadAndDeduplicate_ValidFile 测试读取并去重
func TestReadAndDeduplicate_ValidFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "lines.txt")
	testData := []byte("line1\nline2\nline1\nline3\nline2\n\n")

	// 创建测试文件
	if err := os.WriteFile(testFile, testData, 0666); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 读取并去重
	lines := ReadAndDeduplicate(testFile)

	// 验证去重后的数量
	if len(lines) != 3 {
		t.Errorf("期望 3 条唯一行，实际 %d 条", len(lines))
	}

	// 验证不包含空行
	for _, line := range lines {
		if line == "" {
			t.Errorf("结果中包含空行")
		}
	}
}

// TestReadAndDeduplicate_NonExistingFile 测试读取不存在的文件
func TestReadAndDeduplicate_NonExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "nonexistent.txt")

	// 应该返回 nil
	lines := ReadAndDeduplicate(testFile)
	if lines != nil {
		t.Errorf("ReadAndDeduplicate 应该返回 nil")
	}
}

// TestUniqueLines_BasicDeduplication 测试基本去重
func TestUniqueLines_BasicDeduplication(t *testing.T) {
	str1 := "line1\nline2\nline3"
	str2 := "line2\nline4\nline1"

	result := UniqueLines(str1, str2)

	// 应该有 4 个唯一的行
	if len(result) != 4 {
		t.Errorf("期望 4 条唯一行，实际 %d 条", len(result))
	}

	// 验证结果包含所有行
	resultMap := make(map[string]bool)
	for _, line := range result {
		resultMap[line] = true
	}

	expectedLines := []string{"line1", "line2", "line3", "line4"}
	for _, expected := range expectedLines {
		if !resultMap[expected] {
			t.Errorf("结果中缺少行: %s", expected)
		}
	}
}

// TestUniqueLines_WithWhitespace 测试带空格的行
func TestUniqueLines_WithWhitespace(t *testing.T) {
	str1 := "  line1  \nline2\n  line3  "
	str2 := "line1\n  line2  \nline3"

	result := UniqueLines(str1, str2)

	// 验证结果中的行被正确 trimmed
	for _, line := range result {
		if line != "line1" && line != "line2" && line != "line3" {
			t.Errorf("找到意外的行: %q", line)
		}
	}
}

// TestUniqueLines_EmptyStrings 测试空字符串
func TestUniqueLines_EmptyStrings(t *testing.T) {
	str1 := ""
	str2 := ""

	result := UniqueLines(str1, str2)

	// 应该返回空切片
	if len(result) != 0 {
		t.Errorf("期望返回空切片，实际 %d 条行", len(result))
	}
}

// TestGetFileNamesFolderPath_ValidFolder 测试获取文件夹文件名
func TestGetFileNamesFolderPath_ValidFolder(t *testing.T) {
	tempDir := t.TempDir()

	// 创建一些测试文件
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, file := range testFiles {
		os.WriteFile(filepath.Join(tempDir, file), []byte("content"), 0666)
	}

	// 创建一个子目录
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)

	// 获取文件名
	fileNames, err := GetFileNamesFolderPath(tempDir)
	if err != nil {
		t.Errorf("GetFileNamesFolderPath 失败: %v", err)
	}

	// 验证文件数量（包括子目录）
	if len(fileNames) != 4 {
		t.Errorf("期望 4 个文件/目录，实际 %d 个", len(fileNames))
	}
}

// TestGetFileNamesFolderPath_NonExistingFolder 测试不存在的文件夹
func TestGetFileNamesFolderPath_NonExistingFolder(t *testing.T) {
	tempDir := t.TempDir()
	nonexistentDir := filepath.Join(tempDir, "nonexistent")

	// 应该返回错误
	fileNames, err := GetFileNamesFolderPath(nonexistentDir)
	if err == nil {
		t.Errorf("GetFileNamesFolderPath 应该返回错误")
	}
	if fileNames != nil {
		t.Errorf("GetFileNamesFolderPath 应该返回 nil")
	}
}

// TestGetFileNamesFolderPath_EmptyFolder 测试空文件夹
func TestGetFileNamesFolderPath_EmptyFolder(t *testing.T) {
	tempDir := t.TempDir()

	// 获取文件名
	fileNames, err := GetFileNamesFolderPath(tempDir)
	if err != nil {
		t.Errorf("GetFileNamesFolderPath 失败: %v", err)
	}

	// 验证返回空切片
	if len(fileNames) != 0 {
		t.Errorf("期望返回空切片，实际 %d 个文件", len(fileNames))
	}
}
