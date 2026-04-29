package controller

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	defaultSystemLogTailLines = 500
	maxSystemLogTailLines     = 5000
	maxSystemLogReadBytes     = 8 * 1024 * 1024
)

type SystemLogContentResponse struct {
	Enabled      bool        `json:"enabled"`
	File         LogFileInfo `json:"file"`
	Lines        []string    `json:"lines"`
	LineCount    int         `json:"line_count"`
	Tail         int         `json:"tail"`
	Keyword      string      `json:"keyword,omitempty"`
	Truncated    bool        `json:"truncated"`
	ReadLimitHit bool        `json:"read_limit_hit"`
}

func GetSystemLogFiles(c *gin.Context) {
	if *common.LogDir == "" {
		common.ApiSuccess(c, LogFilesResponse{Enabled: false})
		return
	}
	files, err := getLogFiles()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	common.ApiSuccess(c, LogFilesResponse{
		LogDir:    *common.LogDir,
		Enabled:   true,
		FileCount: len(files),
		TotalSize: totalSize,
		Files:     files,
	})
}

func GetSystemLogContent(c *gin.Context) {
	if *common.LogDir == "" {
		common.ApiSuccess(c, SystemLogContentResponse{Enabled: false})
		return
	}

	tail := parseSystemLogTail(c.Query("tail"))
	fileInfo, filePath, err := resolveSystemLogFile(c.Query("file"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	lines, truncated, readLimitHit, err := readSystemLogTail(filePath, tail)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword != "" {
		lines = filterSystemLogLines(lines, keyword)
	}

	common.ApiSuccess(c, SystemLogContentResponse{
		Enabled:      true,
		File:         fileInfo,
		Lines:        lines,
		LineCount:    len(lines),
		Tail:         tail,
		Keyword:      keyword,
		Truncated:    truncated,
		ReadLimitHit: readLimitHit,
	})
}

func parseSystemLogTail(value string) int {
	tail, err := strconv.Atoi(value)
	if err != nil || tail <= 0 {
		return defaultSystemLogTailLines
	}
	if tail > maxSystemLogTailLines {
		return maxSystemLogTailLines
	}
	return tail
}

func resolveSystemLogFile(fileName string) (LogFileInfo, string, error) {
	files, err := getLogFiles()
	if err != nil {
		return LogFileInfo{}, "", err
	}
	if len(files) == 0 {
		return LogFileInfo{}, "", fmt.Errorf("no log files found")
	}

	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = files[0].Name
	}

	for _, file := range files {
		if file.Name == fileName {
			return file, filepath.Join(*common.LogDir, file.Name), nil
		}
	}
	return LogFileInfo{}, "", fmt.Errorf("invalid log file")
}

func readSystemLogTail(filePath string, tail int) ([]string, bool, bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, false, false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, false, false, err
	}
	if info.Size() == 0 {
		return []string{}, false, false, nil
	}

	const blockSize int64 = 8192
	offset := info.Size()
	var data []byte
	newlineCount := 0
	readBytes := int64(0)

	for offset > 0 && newlineCount <= tail && readBytes < maxSystemLogReadBytes {
		size := blockSize
		if offset < size {
			size = offset
		}
		offset -= size

		block := make([]byte, size)
		if _, err := file.ReadAt(block, offset); err != nil {
			return nil, false, false, err
		}
		newlineCount += bytes.Count(block, []byte{'\n'})
		readBytes += size

		next := make([]byte, 0, len(block)+len(data))
		next = append(next, block...)
		next = append(next, data...)
		data = next
	}

	readLimitHit := offset > 0 && readBytes >= maxSystemLogReadBytes
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	truncated := offset > 0
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
		truncated = true
	}
	return lines, truncated, readLimitHit, nil
}

func filterSystemLogLines(lines []string, keyword string) []string {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return lines
	}
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), keyword) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}
