package workspace

import (
	"context"
	"sync"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/region"
	"github.com/qiniu/qshell/v2/iqshell/common/account"
	"github.com/qiniu/qshell/v2/iqshell/common/config"
	"github.com/qiniu/qshell/v2/iqshell/common/data"
	"github.com/qiniu/qshell/v2/iqshell/common/log"
)

const (
	workspaceName         = ".qshell"
	usersDirName          = "users"
	defaultUserDirName    = ".unknown"
	usersDBName           = "account.db"
	currentUserFileName   = "account.json"
	oldUserFileName       = "old_account.json"
	usersWorkspaceDirName = "workspace"
	taskDirName           = "task"
	taskDBName            = "task.db"
	configFileName        = ".qshell.json"
)

var (
	// config 配置信息
	cfg = &config.Config{}

	// 当前账户
	currentAccount *account.Account

	lock       sync.Mutex
	cancelCtx  context.Context
	cancelFunc func()
)

// GetConfig 获取之前需要先 Load
func GetConfig() *config.Config {
	return cfg
}

func GetLogConfig() *config.LogSetting {
	if cfg == nil || cfg.Log == nil {
		return nil
	}
	return cfg.Log
}

func GetStorageConfig() *storage.Config {
	r := cfg.GetRegion()
	ucHost := cfg.Hosts.GetOneUc()
	if len(ucHost) > 0 {
		log.DebugF("ucHost: %s", ucHost)
		storage.SetUcHost(ucHost, cfg.IsUseHttps())
	}

	return &storage.Config{
		UseHTTPS:      cfg.IsUseHttps(),
		Region:        r,
		Zone:          r,
		CentralRsHost: cfg.Hosts.GetOneRs(),
	}
}

func GetHttpClientOptions() *http_client.Options {
	var options http_client.Options
	options.UseInsecureProtocol = !cfg.IsUseHttps()
	if region := cfg.GetRegion(); region != nil {
		options.Regions = region
	}
	ucHost := cfg.Hosts.GetOneUc()
	if len(ucHost) > 0 {
		log.DebugF("ucHost: %s", ucHost)
		options.SetBucketHosts(region.Endpoints{Preferred: []string{ucHost}})
	}
	return &options
}

func GetAccount() (account.Account, *data.CodeError) {
	if currentAccount == nil {
		return account.Account{}, data.NewEmptyError().AppendDesc("can't get current user")
	}
	return *currentAccount, nil
}

func GetUserName() string {
	if currentAccount == nil {
		return ""
	}
	if len(currentAccount.Name) > 0 {
		return currentAccount.Name
	}
	return currentAccount.AccessKey
}

func GetMac() (mac *qbox.Mac, err *data.CodeError) {
	acc, gErr := GetAccount()
	if gErr != nil {
		err = gErr
		return
	}

	mac = qbox.NewMac(acc.AccessKey, acc.SecretKey)
	return
}

// GetContext 统一使用一个 context
func GetContext() context.Context {
	locker.Lock()
	defer locker.Unlock()
	if cancelCtx != nil {
		return cancelCtx
	}

	cancelCtx, cancelFunc = context.WithCancel(context.Background())
	return cancelCtx
}

func Cancel() {
	if cancelFunc != nil {
		cancelFunc()
	}
}
