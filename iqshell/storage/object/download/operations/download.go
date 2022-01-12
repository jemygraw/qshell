package operations

import (
	"fmt"
	"github.com/qiniu/qshell/v2/iqshell/common/log"
	"github.com/qiniu/qshell/v2/iqshell/common/workspace"
	"github.com/qiniu/qshell/v2/iqshell/storage/object/download"
	"os"
	"time"
)

type DownloadInfo struct {
	download.ApiInfo
	IsPublic bool // 是否是公有云
}

func DownloadFile(info DownloadInfo) {
	_, _ = downloadFile(info)
}

func downloadFile(info DownloadInfo) (string, error) {
	log.InfoF("Download start:%s => %s", info.Url, info.ToFile)

	// 构造下载 url
	if info.IsPublic {
		info.Url = download.PublicUrl(download.UrlApiInfo{
			BucketDomain: info.Domain,
			Key:          info.Key,
			UseHttps:     workspace.GetConfig().IsUseHttps(),
		})
	} else {
		info.Url = download.PrivateUrl(download.UrlApiInfo{
			BucketDomain: info.Domain,
			Key:          info.Key,
			UseHttps:     workspace.GetConfig().IsUseHttps(),
		})
	}

	startTime := time.Now().Unix()
	file, err := download.Download(info.ApiInfo)
	if err != nil {
		log.ErrorF("Download  failed:%s => %s error:%v", info.Url, info.ToFile, err)
		return "", err
	}

	fileStatus, err := os.Stat(file)
	if err != nil {
		log.ErrorF("Download  failed:%s => %s get file status error:%v", info.Url, info.ToFile, err)
		return "", err
	}
	if fileStatus == nil {
		log.ErrorF("Download  failed:%s => %s download speed: can't get file status", info.Url, info.ToFile)
		return "", err
	}

	endTime := time.Now().Unix()

	speed := fmt.Sprintf("%.2fKB/s", float64(fileStatus.Size())/float64(endTime-startTime)/1024)
	log.InfoF("Download success:%s => %s speed:%s", info.Url, file, speed)
	return file, nil
}