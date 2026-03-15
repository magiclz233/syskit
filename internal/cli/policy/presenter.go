// Package policy 中的 presenter 负责把配置/策略文档渲染成 table/markdown。
package policy

import (
	"fmt"
	"io"
	"strings"
	"syskit/internal/errs"
)

// documentSection 表示一段可渲染的文档内容。
// policy 命令的 table/markdown 输出本质上都是由多个 section 组成的。
type documentSection struct {
	Title   string
	Sources []string
	Content string
	Lines   []string
}

// documentPresenter 把多个 section 拼成最终输出。
type documentPresenter struct {
	title    string
	sections []documentSection
}

// newDocumentPresenter 创建一个文档型 presenter。
func newDocumentPresenter(title string, sections []documentSection) *documentPresenter {
	return &documentPresenter{
		title:    title,
		sections: sections,
	}
}

// RenderTable 以可读文本形式输出所有 section。
func (p *documentPresenter) RenderTable(w io.Writer) error {
	if p.title != "" {
		if _, err := fmt.Fprintln(w, p.title); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
		if _, err := fmt.Fprintln(w, strings.Repeat("=", len([]rune(p.title)))); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
	}

	for index, section := range p.sections {
		if err := renderSection(w, section, false); err != nil {
			return err
		}
		if index < len(p.sections)-1 {
			if _, err := fmt.Fprintln(w); err != nil {
				return errs.ExecutionFailed("输出结果失败", err)
			}
		}
	}

	return nil
}

// RenderMarkdown 以 markdown 文档形式输出所有 section。
func (p *documentPresenter) RenderMarkdown(w io.Writer) error {
	if p.title != "" {
		if _, err := fmt.Fprintf(w, "# %s\n\n", p.title); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
	}

	for index, section := range p.sections {
		if _, err := fmt.Fprintf(w, "## %s\n\n", section.Title); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
		if len(section.Sources) > 0 {
			if _, err := fmt.Fprintf(w, "- source: %s\n", strings.Join(section.Sources, ", ")); err != nil {
				return errs.ExecutionFailed("输出结果失败", err)
			}
		}
		for _, line := range section.Lines {
			if _, err := fmt.Fprintf(w, "- %s\n", line); err != nil {
				return errs.ExecutionFailed("输出结果失败", err)
			}
		}
		if section.Content != "" {
			if len(section.Sources) > 0 || len(section.Lines) > 0 {
				if _, err := fmt.Fprintln(w); err != nil {
					return errs.ExecutionFailed("输出结果失败", err)
				}
			}
			if _, err := fmt.Fprintf(w, "```yaml\n%s\n```\n", section.Content); err != nil {
				return errs.ExecutionFailed("输出结果失败", err)
			}
		}
		if index < len(p.sections)-1 {
			if _, err := fmt.Fprintln(w); err != nil {
				return errs.ExecutionFailed("输出结果失败", err)
			}
		}
	}

	return nil
}

// RenderCSV 当前显式拒绝 CSV 输出。
// 原因是 policy/show/init/validate 的结果是文档型结构，强行压成 CSV 会损失大量语义。
func (p *documentPresenter) RenderCSV(w io.Writer, prefix string) error {
	return errs.InvalidArgument("policy 命令暂不支持 csv 输出")
}

// renderSection 是 table 输出的最小渲染单元。
func renderSection(w io.Writer, section documentSection, markdown bool) error {
	if markdown {
		return nil
	}

	if _, err := fmt.Fprintf(w, "[%s]\n", section.Title); err != nil {
		return errs.ExecutionFailed("输出结果失败", err)
	}
	if len(section.Sources) > 0 {
		if _, err := fmt.Fprintf(w, "source: %s\n", strings.Join(section.Sources, ", ")); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
	}
	for _, line := range section.Lines {
		if _, err := fmt.Fprintf(w, "- %s\n", line); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
	}
	if section.Content != "" {
		if len(section.Sources) > 0 || len(section.Lines) > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return errs.ExecutionFailed("输出结果失败", err)
			}
		}
		if _, err := fmt.Fprintln(w, section.Content); err != nil {
			return errs.ExecutionFailed("输出结果失败", err)
		}
	}

	return nil
}
