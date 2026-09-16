package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Markdown -> Word(.docx) 转换器，仅依赖标准库生成合法的 OOXML 文档。

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const docxRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const docxDocumentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
</Relationships>`

const docxNumbering = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="360" w:hanging="180"/></w:pPr></w:lvl>
  </w:abstractNum>
  <w:abstractNum w:abstractNumId="1">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="360" w:hanging="180"/></w:pPr></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
  <w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`

const docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="宋体"/><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
  <w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr><w:rPr><w:b/><w:sz w:val="36"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="200" w:after="100"/></w:pPr><w:rPr><w:b/><w:sz w:val="30"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="160" w:after="80"/></w:pPr><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>
  <w:style w:type="character" w:styleId="QuoteChar"><w:name w:val="Quote Char"/></w:style>
  <w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="360"/></w:pPr><w:rPr><w:i/></w:rPr></w:style>
  <w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/></w:style>
</w:styles>`

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// mdInline 将行内 Markdown（加粗/行内代码/链接）转换为 OOXML runs。
func mdInline(text string) string {
	// 先处理链接 [text](url) -> text
	reLink := regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	text = reLink.ReplaceAllString(text, "$1")
	// 行内代码 `code` -> code（等宽字体由 Word 默认处理）
	reCode := regexp.MustCompile("`([^`]*)`")
	text = reCode.ReplaceAllString(text, "$1")
	// 删除图片语法  ![](...)
	reImg := regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	text = reImg.ReplaceAllString(text, "")

	var b strings.Builder
	// 解析 **bold**
	reBold := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	idx := 0
	for _, m := range reBold.FindAllStringIndex(text, -1) {
		if idx < m[0] {
			b.WriteString("<w:r><w:t xml:space=\"preserve\">" + xmlEscape(text[idx:m[0]]) + "</w:t></w:r>")
		}
		b.WriteString("<w:r><w:rPr><w:b/></w:rPr><w:t xml:space=\"preserve\">" + xmlEscape(text[m[0]+2:m[1]-2]) + "</w:t></w:r>")
		idx = m[1]
	}
	if idx < len(text) {
		b.WriteString("<w:r><w:t xml:space=\"preserve\">" + xmlEscape(text[idx:]) + "</w:t></w:r>")
	}
	return b.String()
}

func mdParagraph(text string) string {
	runs := mdInline(text)
	if runs == "" {
		runs = "<w:r><w:t></w:t></w:r>"
	}
	return "<w:p>" + runs + "</w:p>"
}

func mdHeading(level int, text string) string {
	style := fmt.Sprintf("Heading%d", level)
	return fmt.Sprintf("<w:p><w:pPr><w:pStyle w:val=\"%s\"/></w:pPr>%s</w:p>", style, mdInline(text))
}

func mdBullet(text string) string {
	return "<w:p><w:pPr><w:numPr><w:ilvl w:val=\"0\"/><w:numId w:val=\"1\"/></w:numPr></w:pPr>" + mdInline(text) + "</w:p>"
}

func mdListParagraph(text string) string {
	return "<w:p><w:pPr><w:numPr><w:ilvl w:val=\"0\"/><w:numId w:val=\"2\"/></w:numPr></w:pPr>" + mdInline(text) + "</w:p>"
}

func mdQuote(text string) string {
	return "<w:p><w:pPr><w:pStyle w:val=\"Quote\"/></w:pPr>" + mdInline(text) + "</w:p>"
}

func mdTable(header []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("<w:tbl><w:tblPr>")
	b.WriteString("<w:tblBorders>")
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		b.WriteString(fmt.Sprintf("<w:%s w:val=\"single\" w:sz=\"4\" w:space=\"0\" w:color=\"999999\"/>", edge))
	}
	b.WriteString("</w:tblBorders>")
	b.WriteString("<w:tblW w:w=\"0\" w:type=\"auto\"/></w:tblPr>")

	writeRow := func(cells []string, bold bool) {
		b.WriteString("<w:tr>")
		for _, c := range cells {
			runs := mdInline(strings.TrimSpace(c))
			if bold {
				runs = "<w:r><w:rPr><w:b/></w:rPr><w:t xml:space=\"preserve\">" + xmlEscape(strings.TrimSpace(c)) + "</w:t></w:r>"
			}
			if runs == "" {
				runs = "<w:r><w:t></w:t></w:r>"
			}
			b.WriteString("<w:tc><w:tcPr><w:vAlign w:val=\"center\"/></w:tcPr><w:p>" + runs + "</w:p></w:tc>")
		}
		b.WriteString("</w:tr>")
	}

	writeRow(header, true)
	for _, row := range rows {
		cells := make([]string, len(header))
		copy(cells, header)
		for i := 0; i < len(row) && i < len(cells); i++ {
			cells[i] = row[i]
		}
		writeRow(cells, false)
	}
	b.WriteString("</w:tbl>")
	return b.String()
}

func mdRule() string {
	return "<w:p><w:pPr><w:pBdr><w:bottom w:val=\"single\" w:sz=\"6\" w:space=\"1\" w:color=\"auto\"/></w:pBdr></w:pPr><w:r><w:t></w:t></w:r></w:p>"
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func isTableSeparator(line string) bool {
	l := strings.TrimSpace(strings.Trim(line, "|-: "))
	return l == ""
}

// MarkdownToDocx 将 Markdown 内容转换为标准 .docx 文件。outPath 为输出文件路径。
func MarkdownToDocx(md string, outPath string) error {
	docXML := markdownToDocumentXML(md)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	files := map[string]string{
		"[Content_Types].xml":      docxContentTypes,
		"_rels/.rels":              docxRels,
		"word/_rels/document.xml.rels": docxDocumentRels,
		"word/styles.xml":          docxStyles,
		"word/numbering.xml":       docxNumbering,
		"word/document.xml":        docXML,
	}
	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}

	if err := os.MkdirAll(dirName(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, zipBuf.Bytes(), 0o644)
}

func dirName(path string) string {
	i := strings.LastIndexAny(path, "/\\")
	if i < 0 {
		return "."
	}
	return path[:i]
}

func markdownToDocumentXML(md string) string {
	lines := strings.Split(md, "\n")
	var body strings.Builder

	i := 0
	inCode := false
	var codeBuf strings.Builder
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")

		if inCode {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				body.WriteString(mdParagraph("代码块：\n" + strings.TrimRight(codeBuf.String(), "\n")))
				codeBuf.Reset()
				inCode = false
				i++
				continue
			}
			codeBuf.WriteString(line + "\n")
			i++
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = true
			i++
			continue
		}

		if trimmed == "" {
			i++
			continue
		}

		// 标题
		if n := len(trimmed) - len(strings.TrimLeft(trimmed, "#")); n > 0 && n <= 3 && strings.HasPrefix(strings.TrimLeft(trimmed, "#"), " ") {
			body.WriteString(mdHeading(n, strings.TrimSpace(trimmed[n:])))
			i++
			continue
		}
		// 引用
		if strings.HasPrefix(trimmed, ">") {
			body.WriteString(mdQuote(strings.TrimSpace(trimmed[1:])))
			i++
			continue
		}
		// 分隔线
		if regexp.MustCompile(`^-{3,}$`).MatchString(trimmed) {
			body.WriteString(mdRule())
			i++
			continue
		}
		// 表格：当前行是表头，下一行是分隔线
		if i+1 < len(lines) && strings.HasPrefix(trimmed, "|") && isTableSeparator(lines[i+1]) {
			header := splitRow(trimmed)
			i += 2
			var rows [][]string
			for i < len(lines) {
				t := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
				if t == "" {
					break
				}
				if strings.HasPrefix(t, "|") {
					rows = append(rows, splitRow(t))
					i++
					continue
				}
				break
			}
			body.WriteString(mdTable(header, rows))
			continue
		}
		// 列表（- * 无序）
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			body.WriteString(mdBullet(strings.TrimSpace(trimmed[2:])))
			i++
			continue
		}
		// 有序列表
		if m := regexp.MustCompile(`^\d+[.、]\s+(.*)$`).FindStringSubmatch(trimmed); m != nil {
			body.WriteString(mdListParagraph(m[1]))
			i++
			continue
		}
		// 普通段落
		body.WriteString(mdParagraph(trimmed))
		i++
	}

	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>` + body.String() + `
  <w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>
</w:body>
</w:document>`
}