package ali

import (
	"bufio"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/qiniu/qshell/v2/iqshell/common/log"
	"os"
)

type ListBucketInfo struct {
	DataCenter string
	Bucket     string
	AccessKey  string
	SecretKey  string
	Prefix     string
	SaveToFile string
}

//【alilistbucket】列举阿里空间中的文件列表
func ListBucket(info ListBucketInfo) {

	//open result file
	fp, err := os.Create(info.SaveToFile)
	if err != nil {
		log.Error("create file error:", err)
		return
	}
	defer func(fp *os.File) {
		err := fp.Close()
		if err != nil {
			log.Error("file close error:", err)
		}
	}(fp)

	bw := bufio.NewWriter(fp)
	ossClient, err := oss.New(info.DataCenter, info.AccessKey, info.SecretKey)
	if err != nil {
		log.Error("create oss client error:", err)
		return
	}

	ossBucket, err := ossClient.Bucket(info.Bucket)
	if err != nil {
		log.Error("create oss bucket error:", err)
		return
	}

	log.Info("Listing the oss bucket...")

	var (
		marker        = ""
		limit         = 1000
		retryTimes    = 1
		maxRetryTimes = 5
		prefixLen     = len(info.Prefix)
	)
	for {
		lbr, err := ossBucket.ListObjects(oss.MaxKeys(limit), oss.Prefix(info.Prefix), oss.Marker(marker))
		if err != nil {
			log.Error("Parse list result error,", "marker=[", marker, "]", err)
			if retryTimes <= maxRetryTimes {
				log.Warning("Retry marker=", marker, "] for", retryTimes, "time...")
				retryTimes += 1
				continue
			} else {
				break
			}
		} else {
			retryTimes = 1
		}

		for _, object := range lbr.Objects {
			lmdTime := object.LastModified
			_, err := bw.WriteString(fmt.Sprintln(fmt.Sprintf("%s\t%d\t%d", object.Key[prefixLen:], object.Size, lmdTime.UnixNano()/100)))
			if err != nil {
				log.Error("write result to file:%s error:%v", info.SaveToFile, err)
			}
		}

		if !lbr.IsTruncated {
			break
		}

		marker = lbr.NextMarker
	}

	fErr := bw.Flush()
	if fErr != nil {
		log.Error("Write data to buffer writer failed", fErr)
		err = fErr
		return
	}

	log.Info("List bucket done!")
}