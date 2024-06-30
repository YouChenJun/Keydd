package utils

import (
	logger "Keydd/log"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// 文件是否存在
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// utils, 读取json文件
func ReadJSONFile(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return byteValue, nil
}

// 检查文件夹是否存在
func DirExists(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}
	return true
}

// 复制文件
func CopyFile(sourceFile, destinationFile string) error {
	// 打开源文件
	src, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	// 拷贝文件内容
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return nil
}

// ReadFileData 读取指定文件路径的数据，并返回一个字节切片。
func ReadFileData(filePath string) []byte {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		// 可以选择记录日志或者返回带文件路径的错误
		logger.Error.Printf("error!! reading file %s: %w", filePath, err)
		return nil
	}
	return data
}
func ReadAndDeduplicate(filePath string) []string {
	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		logger.Error.Printf("无法读取文件: %v\n", err)
		return nil
	}

	lines := strings.Split(string(fileData), "\n")
	uniqueLines := make(map[string]struct{}) // 使用map来去重
	for _, line := range lines {
		// 这里假设去重是大小写敏感的，如果不敏感，则可以使用strings.ToLower(line)
		uniqueLines[line] = struct{}{}
	}

	// 将去重后的行转换回切片
	var result []string
	for line := range uniqueLines {
		if line != "" { // 排除空字符串行
			result = append(result, line)
		}
	}
	return result
}

// UniqueLines 接收两个字符串，将它们合并后按行去重，然后返回一个去重后的字符串切片。
func UniqueLines(str1, str2 string) []string {
	linesMap := make(map[string]struct{}) // 创建一个用于存储唯一行的map。
	var result []string                   // 最终去重后的切片。
	// func to process a single string,
	// split into lines, and add to the map.
	processString := func(s string) {
		lines := strings.Split(s, "\n") // 将字符串按行切割。
		for _, line := range lines {
			line = strings.TrimSpace(line) // 去除行首行尾的空格。
			if line != "" {
				linesMap[line] = struct{}{} // 如果行不是空的，则添加到map中。
			}
		}
	}
	// 处理输入的两个字符串。
	processString(str1)
	processString(str2)
	// 将map的键转换成切片。
	for line := range linesMap {
		result = append(result, line)
	}

	return result
}

// 读取文件夹下的全部文件名
func GetFileNamesFolderPath(folderPath string) ([]string, error) {
	// 打开文件系统
	fs, err := os.Open(folderPath)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	// 获取文件信息
	files, err := fs.ReadDir(0)
	if err != nil {
		return nil, err
	}

	// 存储文件名
	fileNames := make([]string, 0)
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}

	return fileNames, nil
}
