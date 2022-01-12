package cmd

import (
	"github.com/qiniu/qshell/v2/iqshell/common/data"
	"github.com/qiniu/qshell/v2/iqshell/storage/object/upload/operations"
	"github.com/spf13/cobra"
)

var uploadCmdBuilder = func() *cobra.Command {
	info := operations.BatchUploadInfo{}
	cmd := &cobra.Command{
		Use:   "qupload <quploadConfigFile>",
		Short: "Batch upload files to the qiniu bucket",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				info.ConfigFile = args[0]
			}
			operations.BatchUpload(info)
		},
	}
	cmd.Flags().StringVarP(&info.SuccessExportFilePath, "success-list", "s", "", "upload success (all) file list")
	cmd.Flags().StringVarP(&info.FailExportFilePath, "failure-list", "f", "", "upload failure file list")
	cmd.Flags().StringVarP(&info.OverrideExportFilePath, "overwrite-list", "w", "", "upload success (overwrite) file list")
	cmd.Flags().Int64VarP(&info.UpThreadCount, "worker", "c", 1, "worker count")
	cmd.Flags().StringVarP(&info.UploadConfig.CallbackUrls, "callback-urls", "l", "", "upload callback urls, separated by comma")
	cmd.Flags().StringVarP(&info.UploadConfig.CallbackHost, "callback-host", "T", "", "upload callback host")
	return cmd
}

var upload2CmdBuilder = func() *cobra.Command {
	info := operations.BatchUploadInfo{}
	cmd := &cobra.Command{
		Use:   "qupload2",
		Short: "Batch upload files to the qiniu bucket",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				info.ConfigFile = args[0]
			}
			operations.BatchUpload(info)
		},
	}
	cmd.Flags().Int64Var(&info.UpThreadCount, "thread-count", 0, "multiple thread count")
	cmd.Flags().BoolVarP(&info.UploadConfig.ResumableAPIV2, "resumable-api-v2", "", false, "use resumable upload v2 APIs to upload")
	cmd.Flags().Int64Var(&info.UploadConfig.ResumableAPIV2PartSize, "resumable-api-v2-part-size", data.BLOCK_SIZE, "the part size when use resumable upload v2 APIs to upload")
	cmd.Flags().StringVar(&info.UploadConfig.SrcDir, "src-dir", "", "src dir to upload")
	cmd.Flags().StringVar(&info.UploadConfig.FileList, "file-list", "", "file list to upload")
	cmd.Flags().StringVar(&info.UploadConfig.Bucket, "bucket", "", "bucket")
	cmd.Flags().Int64Var(&info.UploadConfig.PutThreshold, "put-threshold", 0, "chunk upload threshold")
	cmd.Flags().StringVar(&info.UploadConfig.KeyPrefix, "key-prefix", "", "key prefix prepended to dest file key")
	cmd.Flags().BoolVar(&info.UploadConfig.IgnoreDir, "ignore-dir", false, "ignore the dir in the dest file key")
	cmd.Flags().BoolVar(&info.UploadConfig.Overwrite, "overwrite", false, "overwrite the file of same key in bucket")
	cmd.Flags().BoolVar(&info.UploadConfig.CheckExists, "check-exists", false, "check file key whether in bucket before upload")
	cmd.Flags().BoolVar(&info.UploadConfig.CheckHash, "check-hash", false, "check hash")
	cmd.Flags().BoolVar(&info.UploadConfig.CheckSize, "check-size", false, "check file size")
	cmd.Flags().StringVar(&info.UploadConfig.SkipFilePrefixes, "skip-file-prefixes", "", "skip files with these file prefixes")
	cmd.Flags().StringVar(&info.UploadConfig.SkipPathPrefixes, "skip-path-prefixes", "", "skip files with these relative path prefixes")
	cmd.Flags().StringVar(&info.UploadConfig.SkipFixedStrings, "skip-fixed-strings", "", "skip files with the fixed string in the name")
	cmd.Flags().StringVar(&info.UploadConfig.SkipSuffixes, "skip-suffixes", "", "skip files with these suffixes")
	cmd.Flags().StringVar(&info.UploadConfig.UpHost, "up-host", "", "upload host")
	cmd.Flags().StringVar(&info.UploadConfig.BindUpIp, "bind-up-ip", "", "upload host ip to bind")
	cmd.Flags().StringVar(&info.UploadConfig.BindRsIp, "bind-rs-ip", "", "rs host ip to bind")
	cmd.Flags().StringVar(&info.UploadConfig.BindNicIp, "bind-nic-ip", "", "local network interface card to bind")
	cmd.Flags().BoolVar(&info.UploadConfig.RescanLocal, "rescan-local", false, "rescan local dir to upload newly add files")
	cmd.Flags().StringVar(&info.UploadConfig.LogFile, "log-file", "", "log file")
	cmd.Flags().StringVar(&info.UploadConfig.LogLevel, "log-level", "info", "log level")
	cmd.Flags().IntVar(&info.UploadConfig.LogRotate, "log-rotate", 1, "log rotate days")
	cmd.Flags().IntVar(&info.UploadConfig.FileType, "file-type", 0, "set storage file type")
	cmd.Flags().StringVar(&info.SuccessExportFilePath, "success-list", "", "upload success file list")
	cmd.Flags().StringVar(&info.FailExportFilePath, "failure-list", "", "upload failure file list")
	cmd.Flags().StringVar(&info.OverrideExportFilePath, "overwrite-list", "", "upload success (overwrite) file list")
	cmd.Flags().StringVarP(&info.UploadConfig.CallbackUrls, "callback-urls", "l", "", "upload callback urls, separated by comma")
	cmd.Flags().StringVarP(&info.UploadConfig.CallbackHost, "callback-host", "T", "", "upload callback host")
	return cmd
}

var syncCmdBuilder = func() *cobra.Command {
	info := operations.SyncUploadInfo{}
	cmd := &cobra.Command{
		Use:   "sync <SrcResUrl> <Buckets> [-k <Key>]",
		Short: "Sync big file to qiniu bucket",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				info.ResourceUrl = args[0]
				info.Bucket = args[1]
			}
		},
	}
	cmd.Flags().BoolVarP(&info.IsResumeV2, "resumable-api-v2", "", false, "use resumable upload v2 APIs to upload")
	cmd.Flags().StringVarP(&info.UpHostIp, "uphost", "u", "", "upload host")
	cmd.Flags().StringVarP(&info.Key, "key", "k", "", "save as <key> in bucket")
	return cmd
}

var formUploadCmdBuilder = func() *cobra.Command {
	info := operations.FormUploadInfo{}
	cmd := &cobra.Command{
		Use:   "fput <Bucket> <Key> <LocalFile>",
		Short: "Form upload a local file",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 2 {
				info.Bucket = args[0]
				info.Key = args[1]
				info.FilePath = args[2]
			}
			operations.FormUpload(info)
		},
	}
	cmd.Flags().BoolVarP(&info.Overwrite, "overwrite", "w", false, "overwrite mode")
	cmd.Flags().StringVarP(&info.MimeType, "mimetype", "t", "", "file mime type")
	cmd.Flags().IntVarP(&info.FileType, "storage", "s", 0, "storage type")
	//cmd.Flags().IntVarP(&info.w, "worker", "c", 16, "worker count")
	cmd.Flags().StringVarP(&info.Host, "up-host", "u", "", "uphost")
	cmd.Flags().StringVarP(&info.CallbackUrls, "callback-urls", "l", "", "upload callback urls, separated by comma")
	cmd.Flags().StringVarP(&info.CallbackHost, "callback-host", "T", "", "upload callback host")

	return cmd
}

var resumeUploadCmdBuilder = func() *cobra.Command {
	info := operations.ResumeUploadInfo{}
	cmd := &cobra.Command{
		Use:   "rput <Bucket> <Key> <LocalFile>",
		Short: "Resumable upload a local file",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 2 {
				info.Bucket = args[0]
				info.Key = args[1]
				info.FilePath = args[2]
			}
			operations.ResumeUpload(info)
		},
	}
	cmd.Flags().BoolVarP(&info.IsResumeV2, "v2", "", false, "use resumable upload v2 APIs to upload")
	cmd.Flags().Int64VarP(&info.ResumeV2PartSize, "v2-part-size", "", data.BLOCK_SIZE, "the part size when use resumable upload v2 APIs to upload, default 4M")
	cmd.Flags().BoolVarP(&info.Overwrite, "overwrite", "w", false, "overwrite mode")
	cmd.Flags().StringVarP(&info.MimeType, "mimetype", "t", "", "file mime type")
	cmd.Flags().IntVarP(&info.FileType, "storage", "s", 0, "storage type")
	cmd.Flags().IntVarP(&info.WorkerCount, "worker", "c", 16, "worker count")
	cmd.Flags().StringVarP(&info.Host, "up-host", "u", "", "uphost")
	cmd.Flags().StringVarP(&info.CallbackUrls, "callback-urls", "l", "", "upload callback urls, separated by comma")
	cmd.Flags().StringVarP(&info.CallbackHost, "callback-host", "T", "", "upload callback host")
	return cmd
}

func init() {
	rootCmd.AddCommand(
		uploadCmdBuilder(),
		upload2CmdBuilder(),
		syncCmdBuilder(),
		formUploadCmdBuilder(),
		resumeUploadCmdBuilder(),
	)
}