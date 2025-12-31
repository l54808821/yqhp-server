package utils

import (
	"regexp"
	"strings"
)

// UserAgentInfo User-Agent 解析结果
type UserAgentInfo struct {
	Browser string
	Os      string
}

// ParseUserAgent 解析 User-Agent 字符串
func ParseUserAgent(ua string) UserAgentInfo {
	info := UserAgentInfo{
		Browser: "Unknown",
		Os:      "Unknown",
	}

	if ua == "" {
		return info
	}

	// 解析浏览器
	info.Browser = parseBrowser(ua)

	// 解析操作系统
	info.Os = parseOS(ua)

	return info
}

// parseBrowser 解析浏览器信息
func parseBrowser(ua string) string {
	ua = strings.ToLower(ua)

	// 按优先级检测浏览器（先检测特殊浏览器，再检测通用浏览器）
	browsers := []struct {
		keyword string
		name    string
		version *regexp.Regexp
	}{
		{"edg/", "Edge", regexp.MustCompile(`edg/(\d+)`)},
		{"edge/", "Edge", regexp.MustCompile(`edge/(\d+)`)},
		{"opr/", "Opera", regexp.MustCompile(`opr/(\d+)`)},
		{"opera", "Opera", regexp.MustCompile(`opera/(\d+)`)},
		{"firefox/", "Firefox", regexp.MustCompile(`firefox/(\d+)`)},
		{"chrome/", "Chrome", regexp.MustCompile(`chrome/(\d+)`)},
		{"safari/", "Safari", regexp.MustCompile(`version/(\d+)`)},
		{"msie", "IE", regexp.MustCompile(`msie (\d+)`)},
		{"trident/", "IE", regexp.MustCompile(`rv:(\d+)`)},
	}

	for _, b := range browsers {
		if strings.Contains(ua, b.keyword) {
			if matches := b.version.FindStringSubmatch(ua); len(matches) > 1 {
				return b.name + " " + matches[1]
			}
			return b.name
		}
	}

	return "Unknown"
}

// parseOS 解析操作系统信息
func parseOS(ua string) string {
	// Windows 版本映射
	windowsVersions := map[string]string{
		"windows nt 10.0": "Windows 10/11",
		"windows nt 6.3":  "Windows 8.1",
		"windows nt 6.2":  "Windows 8",
		"windows nt 6.1":  "Windows 7",
		"windows nt 6.0":  "Windows Vista",
		"windows nt 5.1":  "Windows XP",
		"windows nt 5.0":  "Windows 2000",
	}

	uaLower := strings.ToLower(ua)

	// 检测 Windows
	for pattern, name := range windowsVersions {
		if strings.Contains(uaLower, pattern) {
			return name
		}
	}

	// 检测 macOS
	if strings.Contains(uaLower, "mac os x") {
		if matches := regexp.MustCompile(`mac os x (\d+[._]\d+)`).FindStringSubmatch(uaLower); len(matches) > 1 {
			version := strings.ReplaceAll(matches[1], "_", ".")
			return "macOS " + version
		}
		return "macOS"
	}

	// 检测 iOS
	if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") {
		if matches := regexp.MustCompile(`os (\d+[._]\d+)`).FindStringSubmatch(uaLower); len(matches) > 1 {
			version := strings.ReplaceAll(matches[1], "_", ".")
			return "iOS " + version
		}
		return "iOS"
	}

	// 检测 Android
	if strings.Contains(uaLower, "android") {
		if matches := regexp.MustCompile(`android (\d+\.?\d*)`).FindStringSubmatch(uaLower); len(matches) > 1 {
			return "Android " + matches[1]
		}
		return "Android"
	}

	// 检测 Linux
	if strings.Contains(uaLower, "linux") {
		if strings.Contains(uaLower, "ubuntu") {
			return "Ubuntu"
		}
		return "Linux"
	}

	return "Unknown"
}
