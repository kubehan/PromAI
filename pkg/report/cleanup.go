package report

import (
    "log"
    "os"
    "path/filepath"
    "time"
)

// CleanupReports 清理旧报告
func CleanupReports(maxAge int) error {
    reportsDir := "reports"
    now := time.Now()

    // 遍历报告目录
    return filepath.Walk(reportsDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        // 跳过目录本身
        if path == reportsDir {
            return nil
        }

        // 检查文件年龄
		if info.ModTime().Add(time.Duration(maxAge) * 24 * time.Hour).Before(now) {
			if err := os.Remove(path); err != nil {
				log.Printf("删除报告文件失败 %s: %v", path, err)
			} else {
				log.Printf("已删除过期报告: %s", path)
			}
		}

        return nil
    })
}