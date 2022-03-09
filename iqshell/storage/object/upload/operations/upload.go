package operations

import (
	"errors"
	"fmt"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/qshell/v2/iqshell"
	"github.com/qiniu/qshell/v2/iqshell/common/alert"
	"github.com/qiniu/qshell/v2/iqshell/common/config"
	"github.com/qiniu/qshell/v2/iqshell/common/log"
	"github.com/qiniu/qshell/v2/iqshell/common/progress"
	"github.com/qiniu/qshell/v2/iqshell/common/utils"
	"github.com/qiniu/qshell/v2/iqshell/common/workspace"
	"github.com/qiniu/qshell/v2/iqshell/storage/object/upload"
	"os"
	"time"
)

type UploadInfo struct {
	FilePath         string
	Bucket           string
	Key              string
	MimeType         string
	FileStatusDBPath string        // 保存上传状态的 db 文件路径
	FileSize         int64         // 待上传文件的大小, 如果不配置会动态读取 【可选】
	FileModifyTime   int64         // 本地文件修改时间, 如果不配置会动态读取 【可选】
	TokenProvider    func() string // token provider  【可选】
}

func (info *UploadInfo) Check() error {
	if len(info.Bucket) == 0 {
		return alert.CannotEmptyError("Bucket", "")
	}
	if len(info.Key) == 0 && len(info.FilePath) == 0 {
		return alert.CannotEmptyError("Key", "")
	}
	if len(info.FilePath) == 0 {
		return alert.CannotEmptyError("LocalFile", "")
	}
	if utils.IsNetworkSource(info.FilePath) {
		return alert.Error("file can't be network source", "")
	}
	return nil
}

func UploadFile(cfg *iqshell.Config, info UploadInfo) {
	if shouldContinue := iqshell.CheckAndLoad(cfg, iqshell.CheckAndLoadInfo{
		Checker: &info,
	}); !shouldContinue {
		return
	}

	ret, err := uploadFileWithProgress(info, progress.NewPrintProgress(" 进度"))
	if err != nil {
		if v, ok := err.(*storage.ErrorInfo); ok {
			log.ErrorF("Upload file error %d: %s, Reqid: %s", v.Code, v.Err, v.Reqid)
		}
	} else {
		log.Alert("")
		log.Alert("-------------- File Info --------------")
		log.AlertF("%10s%s", "Key: ", ret.Key)
		log.AlertF("%10s%s", "Hash: ", ret.Hash)
		log.AlertF("%10s%d%s", "Fsize: ", ret.FSize, "("+utils.FormatFileSize(ret.FSize)+")")
		log.AlertF("%10s%s", "MimeType: ", ret.MimeType)
	}
}

func uploadFile(info UploadInfo) (res upload.ApiResult, err error) {
	return uploadFileWithProgress(info, nil)
}

func uploadFileWithProgress(info UploadInfo, progress progress.Progress) (res upload.ApiResult, err error) {
	startTime := time.Now().UnixNano() / 1e6
	cfg := workspace.GetConfig()
	uploadConfig := cfg.Up
	apiInfo := upload.ApiInfo{
		FilePath:         info.FilePath,
		ToBucket:         info.Bucket,
		SaveKey:          info.Key,
		MimeType:         info.MimeType,
		FileType:         uploadConfig.FileType.Value(),
		CheckExist:       uploadConfig.IsCheckExists(),
		CheckHash:        uploadConfig.IsCheckHash(),
		CheckSize:        uploadConfig.IsCheckSize(),
		Overwrite:        uploadConfig.IsOverwrite(),
		UpHost:           uploadConfig.UpHost.Value(),
		FileStatusDBPath: info.FileStatusDBPath,
		TokenProvider:    info.TokenProvider,
		TryTimes:         uploadConfig.Retry.Max.Value(),
		TryInterval:      time.Duration(uploadConfig.Retry.Interval.Value()) * time.Millisecond,
		FileSize:         info.FileSize,
		FileModifyTime:   info.FileModifyTime,
		DisableForm:      uploadConfig.IsDisableForm(),
		DisableResume:    uploadConfig.IsDisableResume(),
		UseResumeV2:      uploadConfig.IsResumeAPIV2(),
		ChunkSize:        uploadConfig.ResumableAPIV2PartSize.Value(),
		PutThreshold:     uploadConfig.PutThreshold.Value(),
		Progress:         progress,
	}
	if apiInfo.TokenProvider == nil {
		apiInfo.TokenProvider, err = createTokenProvider(&info)
	}
	if err != nil {
		log.ErrorF("Upload  failed because get token provider error:%s => [%s:%s] error:%v", info.FilePath, info.Bucket, info.Key, err)
		return
	}

	res, err = upload.Upload(apiInfo)
	if err != nil {
		log.ErrorF("Upload  failed:%s => [%s:%s] error:%v", info.FilePath, info.Bucket, info.Key, err)
		return
	}
	endTime := time.Now().UnixNano() / 1e6

	duration := float64(endTime-startTime) / 1000
	speed := fmt.Sprintf("%.2fKB/s", float64(res.FSize)/duration/1024)
	if res.IsSkip {
		log.AlertF("Upload skip because file exist:%s => [%s:%s]", info.FilePath, info.Bucket, info.Key)
	} else {
		log.AlertF("Upload File success %s => [%s:%s] duration:%.2fs Speed:%s", info.FilePath, info.Bucket, info.Key, duration, speed)

		//delete on success
		if uploadConfig.IsDeleteOnSuccess() {
			deleteErr := os.Remove(info.FilePath)
			if deleteErr != nil {
				log.ErrorF("Delete `%s` on upload success error due to `%s`", info.FilePath, deleteErr)
			} else {
				log.InfoF("Delete `%s` on upload success done", info.FilePath)
			}
		}
	}

	return res, nil
}

func createTokenProvider(info *UploadInfo) (provider func() string, err error) {
	mac, gErr := workspace.GetMac()
	if gErr != nil {
		return nil, errors.New("get mac error:" + gErr.Error())
	}

	provider = createTokenProviderWithMac(mac, workspace.GetConfig().Up, info)
	return
}

func createTokenProviderWithMac(mac *qbox.Mac, upConfig *config.Up, info *UploadInfo) func() string {
	policy := *upConfig.Policy
	policy.Scope = info.Bucket
	policy.InsertOnly = 1 // 仅新增不覆盖
	if upConfig.IsOverwrite() {
		policy.Scope = fmt.Sprintf("%s:%s", info.Bucket, info.Key)
		policy.InsertOnly = 0
	}
	policy.ReturnBody = upload.ApiResultFormat()
	policy.FileType = upConfig.FileType.Value()
	return func() string {
		policy.Expires = 7 * 24 * 3600
		return policy.UploadToken(mac)
	}
}
