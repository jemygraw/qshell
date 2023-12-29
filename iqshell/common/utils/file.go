package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/qiniu/qshell/v2/iqshell/common/client"
	"github.com/qiniu/qshell/v2/iqshell/common/data"
)

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)

// FormatFileSize 转化文件大小到人工可读的字符串，以相应的单位显示
func FormatFileSize(size int64) (result string) {
	if size >= TB {
		result = fmt.Sprintf("%.2fTB", float64(size)/float64(TB))
	} else if size >= GB {
		result = fmt.Sprintf("%.2fGB", float64(size)/float64(GB))
	} else if size >= MB {
		result = fmt.Sprintf("%.2fMB", float64(size)/float64(MB))
	} else if size >= KB {
		result = fmt.Sprintf("%.2fKB", float64(size)/float64(KB))
	} else {
		result = fmt.Sprintf("%dB", size)
	}
	return
}

func MarshalToFile(filePath string, v interface{}) *data.CodeError {
	if v == nil {
		return nil
	}

	if err := os.Remove(filePath); err != nil && os.IsExist(err) {
		return data.NewEmptyError().AppendDesc("marshal: delete origin file").AppendError(err)
	}

	err := CreateDirIfNotExist(filepath.Dir(filePath))
	if err != nil {
		return err
	}

	if d, mErr := json.Marshal(v); mErr != nil {
		return data.NewEmptyError().AppendDesc("marshal: marshal").AppendError(mErr)
	} else if wErr := os.WriteFile(filePath, d, os.ModePerm); wErr != nil {
		return data.NewEmptyError().AppendDesc("marshal: write file").AppendError(wErr)
	} else {
		return nil
	}
}

func UnMarshalFromFile(filePath string, v interface{}) *data.CodeError {
	if v == nil {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return data.NewEmptyError().AppendDesc("unmarshal: open file").AppendError(err)
	}
	defer file.Close()

	d, err := ioutil.ReadAll(file)
	if err != nil {
		return data.NewEmptyError().AppendDesc("unmarshal: read file").AppendError(err)
	}

	err = json.Unmarshal(d, v)
	if err != nil {
		return data.NewEmptyError().AppendDesc("unmarshal: unmarshal").AppendError(err)
	}

	return nil
}

func IsNetworkSource(filePath string) bool {
	return strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://")
}

func IsFileMatchFileSize(filePath string, fileSize int64) (match bool, err *data.CodeError) {
	if size, e := FileSize(filePath); e != nil {
		return false, e
	} else if size != fileSize {
		return false, data.NewEmptyError().AppendDescF("size don't match, except:%d but:%d", fileSize, size)
	} else {
		return true, nil
	}
}

func FileSize(filePath string) (fileSize int64, err *data.CodeError) {
	if IsNetworkSource(filePath) {
		return NetworkFileLength(filePath)
	} else {
		return LocalFileSize(filePath)
	}
}

func LocalFileSize(filePath string) (int64, *data.CodeError) {
	fileStatus, err := os.Stat(filePath)
	if err != nil {
		return 0, data.NewEmptyError().AppendDescF("get file size: get status error:%v", err)
	}
	return fileStatus.Size(), nil
}

type NetworkFileInfo struct {
	Size int64
	Hash string
}

func NetworkFileLength(srcResUrl string) (fileSize int64, err *data.CodeError) {
	if f, gErr := GetNetworkFileInfo(srcResUrl); gErr != nil {
		return -1, gErr
	} else {
		return f.Size, nil
	}
}

func GetNetworkFileInfo(srcResUrl string) (*NetworkFileInfo, *data.CodeError) {

	resp, respErr := client.DefaultStorageClient().Head(srcResUrl)
	if respErr != nil {
		return nil, data.NewEmptyError().AppendDescF("New head request failed, %s", respErr.Error())
	}
	defer func() {
		if resp.Body != nil {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	file := &NetworkFileInfo{
		Size: -1,
		Hash: "",
	}
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		if size, pErr := strconv.ParseInt(contentLength, 10, 64); pErr != nil {
			return nil, data.NewEmptyError().AppendDescF("parse network file(%s) size error:%v", srcResUrl, pErr)
		} else {
			file.Size = size
		}
	} else {
		return file, data.NewEmptyError().AppendDescF("network file(%s) hasn't Content-Length", srcResUrl)
	}

	etag := resp.Header.Get("ETag")
	if contentLength != "" {
		file.Hash = ParseEtag(etag)
	} else {
		return nil, data.NewEmptyError().AppendDescF("network file(%s) hasn't Etag", srcResUrl)
	}

	return file, nil
}

func IsLocalFileMatchFileModifyTime(filePath string, modifyTime int64) (match bool, err *data.CodeError) {
	if time, e := LocalFileModify(filePath); e != nil {
		return false, e
	} else if time != modifyTime {
		return false, data.NewEmptyError().AppendDescF("modifyTime don't match, except:%d but:%d", modifyTime, time)
	} else {
		return true, nil
	}
}

func LocalFileModify(filePath string) (int64, *data.CodeError) {
	fileStatus, err := os.Stat(filePath)
	if err != nil {
		return 0, data.NewEmptyError().AppendDescF("get file : get status error:%v", err)
	}
	return fileStatus.ModTime().Unix(), nil
}

func FileLineCounts(filePath string) (count int64, err *data.CodeError) {
	fp, openErr := os.Open(filePath)
	if openErr != nil {
		return 0, data.NewEmptyError().AppendError(openErr)
	}
	defer fp.Close()

	bScanner := bufio.NewScanner(fp)
	for bScanner.Scan() {
		count += 1
	}
	return
}

func CreateFileIfNotExist(path string) *data.CodeError {
	if exist, err := ExistFile(path); err == nil && exist {
		return nil
	}
	return CreateFileDirIfNotExist(path)
}

func CreateFileDirIfNotExist(path string) *data.CodeError {
	dir := filepath.Dir(path)
	if err := CreateDirIfNotExist(dir); err != nil {
		return err
	}
	return nil
}

func ExistFile(path string) (bool, *data.CodeError) {
	if s, err := os.Stat(path); err == nil {
		return !s.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, data.NewEmptyError().AppendError(err)
	}
}

func CreateDirIfNotExist(path string) *data.CodeError {
	if exist, err := ExistDir(path); err == nil && exist {
		return nil
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return data.NewEmptyError().AppendError(err)
	} else {
		return nil
	}
}

func ExistDir(path string) (bool, *data.CodeError) {
	if s, err := os.Stat(path); err == nil {
		return s.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, data.NewEmptyError().AppendError(err)
	}
}
