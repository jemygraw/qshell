package operations

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/qshell/v2/iqshell"
	"github.com/qiniu/qshell/v2/iqshell/common/alert"
	"github.com/qiniu/qshell/v2/iqshell/common/data"
	"github.com/qiniu/qshell/v2/iqshell/common/export"
	"github.com/qiniu/qshell/v2/iqshell/common/flow"
	"github.com/qiniu/qshell/v2/iqshell/common/host"
	"github.com/qiniu/qshell/v2/iqshell/common/locker"
	"github.com/qiniu/qshell/v2/iqshell/common/log"
	"github.com/qiniu/qshell/v2/iqshell/common/utils"
	"github.com/qiniu/qshell/v2/iqshell/common/workspace"
	"github.com/qiniu/qshell/v2/iqshell/storage/object/download"
)

type BatchDownloadWithConfigInfo struct {
	flow.Info
	export.FileExporterConfig

	// 工作数据源
	InputFile    string // 工作数据源：文件
	ItemSeparate string // 工作数据源：每行元素按分隔符分的分隔符

	LocalDownloadConfig string
}

func (info *BatchDownloadWithConfigInfo) Check() *data.CodeError {
	if err := info.Info.Check(); err != nil {
		return err
	}
	if len(info.LocalDownloadConfig) == 0 {
		return alert.CannotEmptyError("LocalDownloadConfig", "")
	}
	return nil
}

func BatchDownloadWithConfig(cfg *iqshell.Config, info BatchDownloadWithConfigInfo) {
	if iqshell.ShowDocumentIfNeeded(cfg) {
		return
	}

	if !iqshell.Check(cfg, iqshell.CheckAndLoadInfo{
		Checker: &info,
	}) {
		return
	}

	downloadInfo := BatchDownloadInfo{
		Info:               info.Info,
		FileExporterConfig: info.FileExporterConfig,
		InputFile:          info.InputFile,
		ItemSeparate:       info.ItemSeparate,
		DownloadCfg:        DefaultDownloadCfg(),
	}
	if err := utils.UnMarshalFromFile(info.LocalDownloadConfig, &downloadInfo.DownloadCfg); err != nil {
		log.ErrorF("UnMarshal: read download config error:%v config file:%s", info.LocalDownloadConfig, err)
		return
	}
	if err := utils.UnMarshalFromFile(info.LocalDownloadConfig, cfg.CmdCfg.Log); err != nil {
		log.ErrorF("UnMarshal: read log setting error:%v config file:%s", info.LocalDownloadConfig, err)
		return
	}
	BatchDownload(cfg, downloadInfo)
}

type BatchDownloadInfo struct {
	flow.Info
	export.FileExporterConfig
	DownloadCfg

	// 工作数据源
	InputFile    string // 工作数据源：文件
	ItemSeparate string // 工作数据源：每行元素按分隔符分的分隔符
}

func (info *BatchDownloadInfo) Check() *data.CodeError {
	if info.WorkerCount < 1 || info.WorkerCount > 2000 {
		log.WarningF("Tip: %d is out of range, you can set <ThreadCount> value between 1 and 200 to improve speed, and now ThreadCount change to: 5",
			info.Info.WorkerCount)
		info.WorkerCount = 5
	}
	if err := info.Info.Check(); err != nil {
		return err
	}
	if err := info.DownloadCfg.Check(); err != nil {
		return err
	}
	if len(info.ItemSeparate) == 0 {
		info.ItemSeparate = data.DefaultLineSeparate
	}
	return nil
}

func BatchDownload(cfg *iqshell.Config, info BatchDownloadInfo) {
	cfg.JobPathBuilder = func(cmdPath string) string {
		if len(info.RecordRoot) > 0 {
			return info.RecordRoot
		}
		return filepath.Join(cmdPath, info.JobId())
	}
	if shouldContinue := iqshell.CheckAndLoad(cfg, iqshell.CheckAndLoadInfo{
		Checker: &info,
	}); !shouldContinue {
		return
	}

	// 配置 locker
	if e := locker.TryLock(); e != nil {
		data.SetCmdStatusError()
		log.ErrorF("Download, %v", e)
		return
	}

	unlockHandler := func() {
		if e := locker.TryUnlock(); e != nil {
			data.SetCmdStatusError()
			log.ErrorF("Download, %v", e)
		}
	}
	workspace.AddCancelObserver(func(s os.Signal) {
		unlockHandler()
	})
	defer unlockHandler()

	info.InputFile = info.KeyFile
	hosts := getDownloadHosts(workspace.GetConfig(), &info.DownloadCfg)
	if len(hosts) == 0 {
		data.SetCmdStatusError()
		log.ErrorF("get download domain error: not find in config and can't get bucket(%s) domain, you can set cdn_domain or bind domain to bucket", info.Bucket)
		return
	}

	dbPath := filepath.Join(workspace.GetJobDir(), ".recorder")
	log.InfoF("download db dir:%s", dbPath)

	exporter, err := export.NewFileExport(export.FileExporterConfig{
		SuccessExportFilePath:   info.SuccessExportFilePath,
		FailExportFilePath:      info.FailExportFilePath,
		OverwriteExportFilePath: info.OverwriteExportFilePath,
	})
	if err != nil {
		log.Error(err)
		data.SetCmdStatusError()
		return
	}

	metric := &Metric{}
	metric.Start()

	hasPrefixes := len(info.Prefix) > 0
	prefixes := strings.Split(info.Prefix, ",")
	filterPrefix := func(name string) bool {
		if !hasPrefixes {
			return false
		}

		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return false
			}
		}
		return true
	}

	hasSuffixes := len(info.Suffixes) > 0
	suffixes := strings.Split(info.Suffixes, ",")
	filterSuffixes := func(name string) bool {
		if !hasSuffixes {
			return false
		}

		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				return false
			}
		}
		return true
	}

	var savePathTemplate *utils.Template
	if len(info.SavePathHandler) > 0 {
		if t, tErr := utils.NewTemplate(info.SavePathHandler); tErr != nil {
			data.SetCmdStatusError()
			log.ErrorF("create save path template fail, %v", savePathTemplate)
			return
		} else {
			savePathTemplate = t
		}
	}

	apiPrefix := ""
	if len(prefixes) == 1 {
		// api 不支持多个 prefix
		apiPrefix = prefixes[0]
	}

	flow.New(info.Info).
		WorkProvider(NewWorkProvider(info.Bucket, apiPrefix, info.InputFile, info.ItemSeparate, func(apiInfo *download.DownloadActionInfo) *data.CodeError {
			apiInfo.Bucket = info.Bucket
			apiInfo.IsPublic = info.Public
			apiInfo.HostProvider = host.NewListProvider(hosts)
			apiInfo.Referer = info.Referer
			apiInfo.FileEncoding = info.FileEncoding
			apiInfo.CheckHash = info.CheckHash
			apiInfo.CheckSize = info.CheckSize
			apiInfo.RemoveTempWhileError = info.RemoveTempWhileError
			apiInfo.UseGetFileApi = info.GetFileApi
			apiInfo.EnableSlice = info.EnableSlice
			apiInfo.SliceSize = info.SliceSize
			apiInfo.SliceConcurrentCount = info.SliceConcurrentCount
			apiInfo.SliceFileSizeThreshold = info.SliceFileSizeThreshold

			apiInfo.DestDir = info.DestDir
			apiInfo.ToFile = filepath.Join(info.DestDir, apiInfo.Key)
			if savePathTemplate != nil {
				if path, rErr := savePathTemplate.Run(apiInfo); rErr != nil {
					return rErr
				} else {
					apiInfo.ToFile = path
				}
			}
			return nil
		})).
		WorkerProvider(flow.NewWorkerProvider(func() (flow.Worker, *data.CodeError) {
			return flow.NewSimpleWorker(func(workInfo *flow.WorkInfo) (flow.Result, *data.CodeError) {
				apiInfo := workInfo.Work.(*download.DownloadActionInfo)
				metric.AddCurrentCount(1)
				metric.PrintProgress("Downloading: " + workInfo.Data)

				if file, e := downloadFile(apiInfo); e != nil {
					return nil, e
				} else {
					log.DebugF("Download Result:%+v", file)
					return file, nil
				}
			}), nil
		})).
		DoWorkListMaxCount(1).
		DoWorkListMinCount(1).
		SetOverseerEnable(true).
		SetDBOverseer(dbPath, func() *flow.WorkRecord {
			return &flow.WorkRecord{
				WorkInfo: &flow.WorkInfo{
					Data: "",
					Work: &download.DownloadActionInfo{},
				},
				Result: &download.DownloadActionResult{},
				Err:    nil,
			}
		}).
		ShouldRedo(func(workInfo *flow.WorkInfo, workRecord *flow.WorkRecord) (shouldRedo bool, cause *data.CodeError) {
			if workRecord.Err != nil {
				return true, workRecord.Err
			}

			apiInfo, _ := workInfo.Work.(*download.DownloadActionInfo)
			recordApiInfo, _ := workRecord.Work.(*download.DownloadActionInfo)

			result, _ := workRecord.Result.(*download.DownloadActionResult)
			if result == nil {
				return true, data.NewEmptyError().AppendDesc("no result found")
			}
			if !result.IsValid() {
				return true, data.NewEmptyError().AppendDescF("result is invalid:%+v", result)
			}

			isLocalFileNotChange, _ := utils.IsLocalFileMatchFileModifyTime(apiInfo.ToFile, result.FileModifyTime)
			isServerFileNotChange := apiInfo.ServerFileHash == recordApiInfo.ServerFileHash
			// 本地文件和服务端文件均没有变化，则不需要重新下载
			if isLocalFileNotChange && isServerFileNotChange {
				return false, nil
			} else if !isLocalFileNotChange {
				// 本地有变动，尝试检查 hash，hash 统一由单文件上传之前检查
				return true, data.NewEmptyError().AppendDesc("local file has change")
			} else {
				// 服务端文件有变动，尝试检查 hash，hash 统一由单文件上传之前检查
				return true, data.NewEmptyError().AppendDesc("server file has change")
			}
		}).
		ShouldSkip(func(workInfo *flow.WorkInfo) (skip bool, cause *data.CodeError) {
			apiInfo, _ := workInfo.Work.(*download.DownloadActionInfo)
			if filterPrefix(apiInfo.Key) {
				//log.InfoF("Download Skip because key prefix doesn't match, [%s:%s]", apiInfo.Bucket, apiInfo.Key)
				return true, data.NewEmptyError().AppendDescF("[%s:%s], prefix filter not match", apiInfo.Bucket, apiInfo.Key)
			}
			if filterSuffixes(apiInfo.Key) {
				//log.InfoF("Download Skip because key suffix doesn't match, [%s:%s]", apiInfo.Bucket, apiInfo.Key)
				return true, data.NewEmptyError().AppendDescF("[%s:%s], suffix filter not match", apiInfo.Bucket, apiInfo.Key)
			}
			return false, nil
		}).
		FlowWillStartFunc(func(flow *flow.Flow) (err *data.CodeError) {
			metric.AddTotalCount(flow.WorkProvider.WorkTotalCount())
			return nil
		}).
		OnWorkSkip(func(workInfo *flow.WorkInfo, result flow.Result, err *data.CodeError) {
			metric.AddCurrentCount(1)
			metric.PrintProgress("Downloading: " + workInfo.Data)

			if err != nil && err.Code == data.ErrorCodeAlreadyDone {
				operationResult, _ := result.(*download.DownloadActionResult)
				if operationResult != nil && operationResult.IsValid() {
					metric.AddSuccessCount(1)
					log.InfoF("Skip line:%s because have done and success", workInfo.Data)
				} else {
					metric.AddFailureCount(1)
					log.InfoF("Skip line:%s because have done and failure, %v", workInfo.Data, err)
				}
			} else {
				metric.AddSkippedCount(1)
				log.InfoF("Skip line:%s because:%v", workInfo.Data, err)
				exporter.Skip().Export(workInfo.Data)
			}
		}).
		OnWorkSuccess(func(workInfo *flow.WorkInfo, result flow.Result) {
			res, _ := result.(*download.DownloadActionResult)
			if res.IsExist {
				metric.AddExistCount(1)
			} else if res.IsUpdate {
				metric.AddUpdateCount(1)
			} else {
				metric.AddSuccessCount(1)
			}

			exporter.Success().Export(workInfo.Data)
		}).
		OnWorkFail(func(workInfo *flow.WorkInfo, err *data.CodeError) {
			metric.AddFailureCount(1)

			exporter.Fail().ExportF("%s%s%s", workInfo.Data, flow.ErrorSeparate, err)
			log.ErrorF("Download  Failed, %s error:%v", workInfo.Data, err)
		}).Build().Start()

	metric.End()
	if metric.TotalCount <= 0 {
		metric.TotalCount = metric.SuccessCount + metric.FailureCount + metric.UpdateCount + metric.ExistCount + metric.SkippedCount
	}

	log.InfoF("job dir:%s, there is a cache related to this command in this folder, which will also be used next time the same command is executed. If you are sure that you don’t need it, you can delete this folder.", workspace.GetJobDir())

	resultPath := filepath.Join(workspace.GetJobDir(), ".result")
	if e := utils.MarshalToFile(resultPath, metric); e != nil {
		data.SetCmdStatusError()
		log.ErrorF("save download result to path:%s error:%v", resultPath, e)
	} else {
		log.DebugF("save download result to path:%s", resultPath)
	}

	log.Info("-------Download Result-------")
	log.InfoF("%10s%10d", "Total:", metric.TotalCount)
	log.InfoF("%10s%10d", "Skipped:", metric.SkippedCount)
	log.InfoF("%10s%10d", "Exists:", metric.ExistCount)
	log.InfoF("%10s%10d", "Success:", metric.SuccessCount)
	log.InfoF("%10s%10d", "Update:", metric.UpdateCount)
	log.InfoF("%10s%10d", "Failure:", metric.FailureCount)
	log.InfoF("%10s%10ds", "Duration:", metric.Duration)
	log.InfoF("-----------------------------")
	if workspace.GetConfig().Log.Enable() {
		log.InfoF("See download log at path:%s", workspace.GetConfig().Log.LogFile.Value())
	}

	if !metric.IsCompletedSuccessfully() {
		data.SetCmdStatusError()
	}
}
