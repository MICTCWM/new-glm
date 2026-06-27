package common

import (
	_ "embed"
	"os"
	"strings"
	"sync"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

//go:embed data/ip2region.xdb
var ip2regionXdbData []byte

var (
	regionSearcher     *xdb.Searcher
	regionSearcherOnce sync.Once
	regionSearcherErr  error
	// regionSearcherMu 保护 searcher.Search 调用，因为 xdb.Searcher 不是线程安全的
	regionSearcherMu sync.Mutex
)

func getRegionSearcher() (*xdb.Searcher, error) {
	regionSearcherOnce.Do(func() {
		header, err := xdb.LoadHeaderFromBuff(ip2regionXdbData)
		if err != nil {
			regionSearcherErr = err
			return
		}
		version, err := xdb.VersionFromHeader(header)
		if err != nil {
			regionSearcherErr = err
			return
		}
		searcher, err := xdb.NewWithBuffer(version, ip2regionXdbData)
		if err != nil {
			regionSearcherErr = err
			return
		}
		regionSearcher = searcher
	})
	return regionSearcher, regionSearcherErr
}

// IsRegionBlockEnabled 检查是否启用中国 IP 拦截
// 默认启用，设置 BLOCK_CN_REGION=false 或 0 可关闭
func IsRegionBlockEnabled() bool {
	v := strings.ToLower(os.Getenv("BLOCK_CN_REGION"))
	return v != "false" && v != "0"
}

// IsChinaIP 判断 IP 是否属于中国大陆
func IsChinaIP(ipStr string) bool {
	if ipStr == "" {
		return false
	}
	ip := ParseIP(ipStr)
	if ip == nil {
		return false
	}
	// 私有 IP 不拦截
	if IsPrivateIP(ip) {
		return false
	}
	searcher, err := getRegionSearcher()
	if err != nil || searcher == nil {
		return false
	}
	// xdb.Searcher.Search 非线程安全（会写入共享字段 s.ioCount），需加互斥锁
	regionSearcherMu.Lock()
	region, err := searcher.Search(ipStr)
	regionSearcherMu.Unlock()
	if err != nil {
		return false
	}
	// ip2region 返回格式：国家|区域|省份|城市|ISP
	// 中国大陆的国家字段为 "中国"
	parts := strings.Split(region, "|")
	if len(parts) > 0 {
		country := strings.TrimSpace(parts[0])
		return country == "中国"
	}
	return false
}
