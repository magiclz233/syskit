package policy

import (
	"fmt"
	"io"
	"strings"
	"syskit/internal/errs"
)

type documentSection struct {
	Title   string
	Sources []string
	Content string
	Lines   []string
}

type documentPresenter struct {
	title    string
	sections []documentSection
}

func newDocumentPresenter(title string, sections []documentSection) *documentPresenter {
	return &documentPresenter{
		title:    title,
		sections: sections,
	}
}

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

func (p *documentPresenter) RenderCSV(w io.Writer, prefix string) error {
	return errs.InvalidArgument("policy 命令暂不支持 csv 输出")
}

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
