package utils

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var globalPort string

func GetLocalIPs() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var ips []string
	for _, iface := range interfaces {
		// 跳过禁用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// 获取接口的所有地址
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				// 排除回环地址和 IPv6 地址
				if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}
	return ips
}

func SetGlobalPort(port string) {
	globalPort = port
}

func GetGlobalPort() string {
	return globalPort
}

// GetServerURL 获取服务器的基础URL，支持动态域名获取
func GetServerURL(r *http.Request) string {
	// log.Printf("打印请求信息: %s", r)
	// 从请求中获取scheme (http/https)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// 检查X-Forwarded-Proto头（适用于反向代理场景）
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	// 获取Host
	host := r.Host

	// 如果Host不包含端口，尝试从环境变量获取外部访问端口
	if !strings.Contains(host, ":") {
		// 优先从环境变量获取外部端口
		externalPort := os.Getenv("EXTERNAL_PORT")
		if externalPort != "" {
			host = host + ":" + externalPort
			log.Printf("EXTERNAL_PORT不为空,从环境变量获取外部端口: %s", host)
		} else if globalPort != "" && globalPort != "80" && globalPort != "443" {
			// 其次使用服务运行端口（如果不是标准端口）
			host = host + ":" + globalPort
			log.Printf("EXTERNAL_PORT为空,使用服务运行端口: %s", host)
		}
	}

	log.Printf("GetServerURL调试: 最终Host: %s", host)

	return scheme + "://" + host
}

// GetReportURL 获取报告访问URL
func GetReportURL(r *http.Request, reportFileName string) string {
	serverURL := GetServerURL(r)
	return strings.TrimRight(serverURL, "/") + "/api/promai/reports/" + url.PathEscape(reportFileName)
}

// GetServerURLFromContext 从配置中获取服务器URL
// 用于定时任务等没有HTTP请求的场景
func GetServerURLFromContext(configReportURL string) string {
	// 如果配置了环境变量，优先使用环境变量
	if envURL := os.Getenv("REPORT_URL"); envURL != "" {
		return envURL
	}

	// 如果有配置文件中的report_url，使用配置的
	if configReportURL != "" {
		return configReportURL
	}

	// 默认使用localhost + 端口
	scheme := "http"
	port := globalPort
	if port == "" {
		port = "80" // 默认端口
	}

	// 如果是标准端口，可以省略
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return scheme + "://localhost"
	}

	return scheme + "://localhost:" + port

}

func BuildReportURL(configReportURL, reportFileName string) string {
	baseURL := strings.TrimRight(GetServerURLFromContext(configReportURL), "/")
	return baseURL + "/api/promai/reports/" + url.PathEscape(reportFileName)
}
