package tools

import (
	"fmt"
	"strings"
)

// detectLineEnding 检测行结束符
func detectLineEnding(content string) string {
	crlfIdx := strings.Index(content, "\r\n")
	lfIdx := strings.Index(content, "\n")
	if lfIdx == -1 {
		return "\n"
	}
	if crlfIdx == -1 {
		return "\n"
	}
	if crlfIdx < lfIdx {
		return "\r\n"
	}
	return "\n"
}

// normalizeToLF 规范化为 LF
func normalizeToLF(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// restoreLineEndings 恢复行结束符
func restoreLineEndings(text string, ending string) string {
	if ending == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// normalizeForFuzzyMatch 规范化文本用于模糊匹配
func normalizeForFuzzyMatch(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	text = strings.Join(lines, "\n")

	text = strings.Map(func(r rune) rune {
		switch r {
		case '\u2018', '\u2019', '\u201A', '\u201B':
			return '\''
		case '\u201C', '\u201D', '\u201E', '\u201F':
			return '"'
		case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212':
			return '-'
		case '\u00A0', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200A', '\u202F', '\u205F', '\u3000':
			return ' '
		default:
			return r
		}
	}, text)

	return text
}

// FuzzyMatchResult 模糊匹配结果
type FuzzyMatchResult struct {
	Found               bool
	Index               int
	MatchLength         int
	UsedFuzzyMatch      bool
	ContentForReplacement string
}

// fuzzyFindText 模糊查找文本
func fuzzyFindText(content string, oldText string) FuzzyMatchResult {
	exactIndex := strings.Index(content, oldText)
	if exactIndex != -1 {
		return FuzzyMatchResult{
			Found:               true,
			Index:               exactIndex,
			MatchLength:         len(oldText),
			UsedFuzzyMatch:      false,
			ContentForReplacement: content,
		}
	}

	fuzzyContent := normalizeForFuzzyMatch(content)
	fuzzyOldText := normalizeForFuzzyMatch(oldText)
	fuzzyIndex := strings.Index(fuzzyContent, fuzzyOldText)

	if fuzzyIndex == -1 {
		return FuzzyMatchResult{
			Found:               false,
			Index:               -1,
			MatchLength:         0,
			UsedFuzzyMatch:      false,
			ContentForReplacement: content,
		}
	}

	return FuzzyMatchResult{
		Found:               true,
		Index:               fuzzyIndex,
		MatchLength:         len(fuzzyOldText),
		UsedFuzzyMatch:      true,
		ContentForReplacement: fuzzyContent,
	}
}

// stripBom 移除 BOM
func stripBom(content string) (string, string) {
	if strings.HasPrefix(content, "\uFEFF") {
		return "\uFEFF", content[1:]
	}
	return "", content
}

// DiffPart 差异部分
type DiffPart struct {
	Value   string
	Added   bool
	Removed bool
}

// DiffResult 差异结果
type DiffResult struct {
	Diff             string
	FirstChangedLine int
}

// generateDiffString 生成差异字符串
func generateDiffString(oldContent string, newContent string, contextLines ...int) DiffResult {
	ctxLines := 4
	if len(contextLines) > 0 {
		ctxLines = contextLines[0]
	}

	parts := diffLines(oldContent, newContent)
	output := []string{}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	maxLineNum := len(oldLines)
	if len(newLines) > maxLineNum {
		maxLineNum = len(newLines)
	}
	lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))

	oldLineNum := 1
	newLineNum := 1
	lastWasChange := false
	var firstChangedLine *int

	for i := 0; i < len(parts); i++ {
		part := parts[i]
		raw := strings.Split(part.Value, "\n")
		if len(raw) > 0 && raw[len(raw)-1] == "" {
			raw = raw[:len(raw)-1]
		}

		if part.Added || part.Removed {
			if firstChangedLine == nil {
				firstChangedLine = &newLineNum
			}

			for _, line := range raw {
				if part.Added {
					lineNum := fmt.Sprintf("%*d", lineNumWidth, newLineNum)
					output = append(output, fmt.Sprintf("+%s %s", lineNum, line))
					newLineNum++
				} else {
					lineNum := fmt.Sprintf("%*d", lineNumWidth, oldLineNum)
					output = append(output, fmt.Sprintf("-%s %s", lineNum, line))
					oldLineNum++
				}
			}
			lastWasChange = true
		} else {
			nextPartIsChange := i < len(parts)-1 && (parts[i+1].Added || parts[i+1].Removed)

			if lastWasChange || nextPartIsChange {
				linesToShow := raw
				skipStart := 0
				skipEnd := 0

				if !lastWasChange {
					skipStart = max(0, len(raw)-ctxLines)
					linesToShow = raw[skipStart:]
				}

				if !nextPartIsChange && len(linesToShow) > ctxLines {
					skipEnd = len(linesToShow) - ctxLines
					linesToShow = linesToShow[:ctxLines]
				}

				if skipStart > 0 {
					output = append(output, fmt.Sprintf(" %s ...", strings.Repeat(" ", lineNumWidth)))
					oldLineNum += skipStart
					newLineNum += skipStart
				}

				for _, line := range linesToShow {
					lineNum := fmt.Sprintf("%*d", lineNumWidth, oldLineNum)
					output = append(output, fmt.Sprintf(" %s %s", lineNum, line))
					oldLineNum++
					newLineNum++
				}

				if skipEnd > 0 {
					output = append(output, fmt.Sprintf(" %s ...", strings.Repeat(" ", lineNumWidth)))
					oldLineNum += skipEnd
					newLineNum += skipEnd
				}
			} else {
				oldLineNum += len(raw)
				newLineNum += len(raw)
			}

			lastWasChange = false
		}
	}

	result := DiffResult{
		Diff: strings.Join(output, "\n"),
	}
	if firstChangedLine != nil {
		result.FirstChangedLine = *firstChangedLine
	}

	return result
}

// diffLines 计算行差异
func diffLines(oldContent string, newContent string) []DiffPart {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var parts []DiffPart

	lcs := longestCommonSubsequence(oldLines, newLines)

	oldIdx := 0
	newIdx := 0
	lcsIdx := 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if lcsIdx < len(lcs) {
			if oldIdx < len(oldLines) && oldLines[oldIdx] == lcs[lcsIdx] {
				if newIdx < len(newLines) && newLines[newIdx] == lcs[lcsIdx] {
					if len(parts) > 0 && !parts[len(parts)-1].Added && !parts[len(parts)-1].Removed {
						parts[len(parts)-1].Value += "\n" + lcs[lcsIdx]
					} else {
						parts = append(parts, DiffPart{Value: lcs[lcsIdx]})
					}
					oldIdx++
					newIdx++
					lcsIdx++
					continue
				}
			}
		}

		if oldIdx < len(oldLines) && (lcsIdx >= len(lcs) || oldLines[oldIdx] != lcs[lcsIdx]) {
			if len(parts) > 0 && parts[len(parts)-1].Removed {
				parts[len(parts)-1].Value += "\n" + oldLines[oldIdx]
			} else {
				parts = append(parts, DiffPart{Value: oldLines[oldIdx], Removed: true})
			}
			oldIdx++
		}

		if newIdx < len(newLines) && (lcsIdx >= len(lcs) || newLines[newIdx] != lcs[lcsIdx]) {
			if len(parts) > 0 && parts[len(parts)-1].Added {
				parts[len(parts)-1].Value += "\n" + newLines[newIdx]
			} else {
				parts = append(parts, DiffPart{Value: newLines[newIdx], Added: true})
			}
			newIdx++
		}
	}

	return parts
}

// longestCommonSubsequence 最长公共子序列
func longestCommonSubsequence(a []string, b []string) []string {
	m := len(a)
	n := len(b)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	result := []string{}
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([]string{a[i-1]}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return result
}