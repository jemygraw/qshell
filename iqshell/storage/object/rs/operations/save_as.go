package operations

import (
	"github.com/qiniu/qshell/v2/iqshell/common/data"
	"github.com/qiniu/qshell/v2/iqshell/common/log"
	"github.com/qiniu/qshell/v2/iqshell/storage/object/rs"
	"os"
)

type SaveAsInfo rs.SaveAsApiInfo

func SaveAs(info SaveAsInfo) {
	url, err := rs.SaveAs(rs.SaveAsApiInfo(info))
	if err != nil {
		log.ErrorF("save as error: %v", err)
		os.Exit(data.STATUS_ERROR)
	} else {
		log.Alert(url)
	}
}