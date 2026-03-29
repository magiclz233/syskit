package filecmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	filecollector "syskit/internal/collectors/file"
	"syskit/internal/errs"
	"syskit/pkg/utils"
)

type dupPresenter struct {
	data *filecollector.DupResult
}

func newDupPresenter(data *filecollector.DupResult) *dupPresenter {
	return &dupPresenter{data: data}
}

func (p *dupPresenter) RenderTable(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file dup 输出结果为空")
	}
	fmt.Fprintf(w, "重复文件检测结果 path=%s hash=%s\n", p.data.Path, p.data.Hash)
	fmt.Fprintf(w, "groups=%d duplicates=%d wasted=%s scanned=%d\n", p.data.GroupCount, p.data.DuplicateCount, utils.FormatBytes(p.data.WastedBytes), p.data.ScannedFiles)
	for _, group := range p.data.Groups {
		fmt.Fprintf(w, "\n[%s] size=%s count=%d\n", group.Hash, utils.FormatBytes(group.SizeBytes), group.Count)
		for _, file := range group.Files {
			fmt.Fprintf(w, "- %s\n", file.Path)
		}
	}
	renderWarnings(w, p.data.Warnings)
	return nil
}

func (p *dupPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file dup 输出结果为空")
	}
	fmt.Fprintln(w, "# File Dup")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- path: `%s`\n", mdCell(p.data.Path))
	fmt.Fprintf(w, "- hash: `%s`\n", p.data.Hash)
	fmt.Fprintf(w, "- group_count: `%d`\n", p.data.GroupCount)
	fmt.Fprintf(w, "- duplicate_count: `%d`\n", p.data.DuplicateCount)
	fmt.Fprintf(w, "- wasted_bytes: `%d`\n", p.data.WastedBytes)
	if len(p.data.Groups) > 0 {
		fmt.Fprintln(w, "\n| HASH | SIZE_BYTES | COUNT |")
		fmt.Fprintln(w, "|---|---:|---:|")
		for _, group := range p.data.Groups {
			fmt.Fprintf(w, "| %s | %d | %d |\n", mdCell(group.Hash), group.SizeBytes, group.Count)
		}
	}
	if len(p.data.Warnings) > 0 {
		fmt.Fprintln(w, "\n## Warnings")
		fmt.Fprintln(w)
		for _, warning := range p.data.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}
	return nil
}

func (p *dupPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file dup 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"hash", "size_bytes", "count", "path"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, group := range p.data.Groups {
		for _, file := range group.Files {
			row := []string{
				group.Hash,
				strconv.FormatInt(group.SizeBytes, 10),
				strconv.Itoa(group.Count),
				file.Path,
			}
			if err := writer.Write(row); err != nil {
				return errs.ExecutionFailed("写入 CSV 内容失败", err)
			}
		}
	}
	return nil
}

type archivePresenter struct {
	data *archiveOutputData
}

func newArchivePresenter(data *archiveOutputData) *archivePresenter {
	return &archivePresenter{data: data}
}

func (p *archivePresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file archive 输出结果为空")
	}
	fmt.Fprintf(w, "File Archive mode=%s path=%s\n", p.data.Mode, p.data.Plan.Path)
	fmt.Fprintf(w, "archive_path=%s compress=%s candidates=%d\n", p.data.Plan.ArchivePath, p.data.Plan.Compress, p.data.Plan.CandidateCount)
	if p.data.Result != nil {
		fmt.Fprintf(w, "archived=%d removed=%d failed=%d\n", p.data.Result.ArchivedCount, p.data.Result.RemovedCount, len(p.data.Result.Failed))
	}
	renderWarnings(w, p.data.Plan.Warnings)
	if p.data.Result != nil {
		renderWarnings(w, p.data.Result.Warnings)
	}
	return nil
}

func (p *archivePresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file archive 输出结果为空")
	}
	fmt.Fprintln(w, "# File Archive")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- path: `%s`\n", mdCell(p.data.Plan.Path))
	fmt.Fprintf(w, "- archive_path: `%s`\n", mdCell(p.data.Plan.ArchivePath))
	fmt.Fprintf(w, "- compress: `%s`\n", p.data.Plan.Compress)
	fmt.Fprintf(w, "- candidate_count: `%d`\n", p.data.Plan.CandidateCount)
	if p.data.Result != nil {
		fmt.Fprintln(w, "\n## Result")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "- archived_count: `%d`\n", p.data.Result.ArchivedCount)
		fmt.Fprintf(w, "- removed_count: `%d`\n", p.data.Result.RemovedCount)
		fmt.Fprintf(w, "- failed_count: `%d`\n", len(p.data.Result.Failed))
	}
	return nil
}

func (p *archivePresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file archive 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"row_type", "source_path", "size_bytes", "mode", "status"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	status := "planned"
	if p.data.Result != nil {
		status = "applied"
	}
	for _, item := range p.data.Plan.Candidates {
		if err := writer.Write([]string{"candidate", item.SourcePath, strconv.FormatInt(item.SizeBytes, 10), p.data.Mode, status}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

type emptyPresenter struct {
	data *emptyOutputData
}

func newEmptyPresenter(data *emptyOutputData) *emptyPresenter {
	return &emptyPresenter{data: data}
}

func (p *emptyPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file empty 输出结果为空")
	}
	fmt.Fprintf(w, "File Empty mode=%s path=%s candidates=%d\n", p.data.Mode, p.data.Plan.Path, p.data.Plan.CandidateCount)
	if p.data.Result != nil {
		fmt.Fprintf(w, "deleted=%d failed=%d\n", p.data.Result.DeletedDirs, len(p.data.Result.Failed))
	}
	for _, dir := range p.data.Plan.EmptyDirs {
		fmt.Fprintf(w, "- %s\n", dir)
	}
	renderWarnings(w, p.data.Plan.Warnings)
	if p.data.Result != nil {
		renderWarnings(w, p.data.Result.Warnings)
	}
	return nil
}

func (p *emptyPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file empty 输出结果为空")
	}
	fmt.Fprintln(w, "# File Empty")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- path: `%s`\n", mdCell(p.data.Plan.Path))
	fmt.Fprintf(w, "- candidate_count: `%d`\n", p.data.Plan.CandidateCount)
	return nil
}

func (p *emptyPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file empty 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"dir", "mode"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, dir := range p.data.Plan.EmptyDirs {
		if err := writer.Write([]string{dir, p.data.Mode}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

type dedupPresenter struct {
	data *dedupOutputData
}

func newDedupPresenter(data *dedupOutputData) *dedupPresenter {
	return &dedupPresenter{data: data}
}

func (p *dedupPresenter) RenderTable(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file dedup 输出结果为空")
	}
	fmt.Fprintf(w, "File Dedup mode=%s path=%s delete=%d (%s)\n", p.data.Mode, p.data.Plan.Path, len(p.data.Plan.DeleteFiles), utils.FormatBytes(p.data.Plan.DeleteBytes))
	if p.data.Result != nil {
		fmt.Fprintf(w, "deleted=%d failed=%d\n", p.data.Result.DeletedFiles, len(p.data.Result.Failed))
	}
	for _, path := range p.data.Plan.DeleteFiles {
		fmt.Fprintf(w, "- %s\n", path)
	}
	renderWarnings(w, p.data.Plan.Warnings)
	if p.data.Result != nil {
		renderWarnings(w, p.data.Result.Warnings)
	}
	return nil
}

func (p *dedupPresenter) RenderMarkdown(w io.Writer) error {
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file dedup 输出结果为空")
	}
	fmt.Fprintln(w, "# File Dedup")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- mode: `%s`\n", p.data.Mode)
	fmt.Fprintf(w, "- path: `%s`\n", mdCell(p.data.Plan.Path))
	fmt.Fprintf(w, "- delete_count: `%d`\n", len(p.data.Plan.DeleteFiles))
	fmt.Fprintf(w, "- delete_bytes: `%d`\n", p.data.Plan.DeleteBytes)
	return nil
}

func (p *dedupPresenter) RenderCSV(w io.Writer, prefix string) error {
	_ = prefix
	if p.data == nil || p.data.Plan == nil {
		return errs.New(errs.ExitExecutionFailed, errs.CodeExecutionFailed, "file dedup 输出结果为空")
	}
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"path", "mode"}); err != nil {
		return errs.ExecutionFailed("写入 CSV 表头失败", err)
	}
	for _, path := range p.data.Plan.DeleteFiles {
		if err := writer.Write([]string{path, p.data.Mode}); err != nil {
			return errs.ExecutionFailed("写入 CSV 内容失败", err)
		}
	}
	return nil
}

func renderWarnings(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n提示")
	for _, item := range warnings {
		fmt.Fprintf(w, "- %s\n", item)
	}
}

func mdCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
