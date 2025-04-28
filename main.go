package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	router "github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

var (
	urlsFile   = flag.String("urlfile", "urls.txt", "包含域名列表URL的文件路径")
	outputName = flag.String("outputname", "geosite.dat", "生成的dat文件名")
	outputDir  = flag.String("outputdir", "./output", "输出目录")
)

type Entry struct {
	Type  string
	Value string
	Attrs []*router.Domain_Attribute
}

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// 检查字符串是否是一个有效的域名
func isValidDomain(domain string) bool {
	// 域名总长度不能超过 253 个字符
	if len(domain) > 253 {
		return false
	}
	// 使用正则表达式进行结构校验
	return domainRegex.MatchString(domain)
}

// 从URL下载域名列表，并校验域名格式
func downloadList(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载列表失败 %s: %w", url, err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载列表失败 %s: 状态码 %d", url, resp.StatusCode)
	}

	var domains []string
	scanner := bufio.NewScanner(resp.Body)
	lineNumber := 0 // 用于定位无效行

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "//") || strings.HasPrefix(line, "!") {
			continue
		}

		// 转换为小写进行校验和存储
		domainLower := strings.ToLower(line)

		// 校验是否为有效域名格式
		if isValidDomain(domainLower) {
			domains = append(domains, domainLower) // 添加校验通过的域名（小写）
		} else {
			// 报告无效的域名行
			fmt.Printf("警告: 在第 %d 行跳过无效域名: %s\n", lineNumber, line)
		}
	}

	if err := scanner.Err(); err != nil {
		// 返回已成功解析的部分以及扫描错误
		return domains, fmt.Errorf("读取响应体时出错 %s: %w", url, err)
	}

	return domains, nil
}

// 定义URL列表项结构
type URLItem struct {
	Prefix string
	URL    string
}

// 解析URL列表行
func parseURLLine(line string) (*URLItem, error) {
	parts := strings.Split(line, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的URL配置格式，应为: prefix,url，实际为: %s", line)
	}

	prefix := strings.TrimSpace(parts[0])
	url := strings.TrimSpace(parts[1])

	if prefix == "" {
		return nil, errors.New("前缀不能为空")
	}
	if url == "" {
		return nil, errors.New("URL不能为空")
	}

	return &URLItem{
		Prefix: strings.ToUpper(prefix),
		URL:    url,
	}, nil
}

// 用于传递下载结果的结构
type DownloadResult struct {
	Prefix  string
	Domains []string
	Err     error
}

// 处理单个 URL 下载并在 channel 中发送结果
func processURL(urlItem *URLItem, resultChan chan<- DownloadResult, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("正在处理列表 %s: %s\n", urlItem.Prefix, urlItem.URL)

	domains, err := downloadList(urlItem.URL)

	resultChan <- DownloadResult{
		Prefix:  urlItem.Prefix,
		Domains: domains,
		Err:     err,
	}
}

func main() {
	flag.Parse()

	// 读取URL列表文件
	urlFile, err := os.Open(*urlsFile)
	if err != nil {
		fmt.Printf("无法打开URL文件: %v\n", err)
		os.Exit(1)
	}
	defer urlFile.Close()

	// 创建输出目录
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	// 使用 map 来存储下载结果，方便按前缀查找和组织
	downloadedSites := make(map[string][]string)
	var processingErrors []error // 收集处理过程中的错误

	// 读取URL列表并启动goroutines
	scanner := bufio.NewScanner(urlFile)
	var wg sync.WaitGroup
	resultChan := make(chan DownloadResult)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析URL配置行
		urlItem, err := parseURLLine(line)
		if err != nil {
			fmt.Printf("解析配置失败: %v\n", err)
			processingErrors = append(processingErrors, fmt.Errorf("解析配置失败: %w", err))
			continue
		}

		wg.Add(1)
		go processURL(urlItem, resultChan, &wg)
	}

	// 等待所有goroutines完成并关闭channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 从channel中接收结果
	for result := range resultChan {
		if result.Err != nil {
			fmt.Printf("下载列表失败 %s: %v\n", result.Prefix, result.Err)
			processingErrors = append(processingErrors, fmt.Errorf("下载列表 %s 失败: %w", result.Prefix, result.Err))
		} else {
			downloadedSites[result.Prefix] = result.Domains
		}
	}

	// 检查是否有处理或下载错误，如果有，则 panic
	if len(processingErrors) > 0 {
		panic(fmt.Errorf("处理过程中发生错误: %v", processingErrors))
	}

	// 将下载结果转换为GeoSiteList
	protoList := new(router.GeoSiteList)
	// 按照前缀（CountryCode）排序，以便 consistent output
	prefixes := make([]string, 0, len(downloadedSites))
	for prefix := range downloadedSites {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	for _, prefix := range prefixes {
		domains := downloadedSites[prefix]
		site := &router.GeoSite{
			CountryCode: prefix,
			Domain:      make([]*router.Domain, 0, len(domains)),
		}
		// 将域名转换为Domain结构
		for _, domain := range domains {
			site.Domain = append(site.Domain, &router.Domain{
				Type:  router.Domain_RootDomain,
				Value: domain,
			})
		}
		protoList.Entry = append(protoList.Entry, site)
	}

	// 生成dat文件
	protoBytes, err := proto.Marshal(protoList)
	if err != nil {
		fmt.Printf("序列化失败: %v\n", err)
		// 序列化失败直接 panic
		panic(fmt.Errorf("序列化 GeoSiteList 失败: %w", err))
	}

	outPath := filepath.Join(*outputDir, *outputName)
	if err := os.WriteFile(outPath, protoBytes, 0644); err != nil {
		fmt.Printf("写入文件失败: %v\n", err)
		// 写入文件失败直接 panic
		panic(fmt.Errorf("写入 GeoSite 文件失败: %w", err))
	}

	fmt.Printf("成功生成 %s\n", *outputName)
}
