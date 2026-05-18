package paidscan

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

const DefaultMaxFiles = 100
const DefaultMaxFileBytes = 50 * 1024 * 1024
const DefaultMaxTotalBytes = 250 * 1024 * 1024

type Options struct {
	MaxFiles      int
	MaxFileBytes  int64
	MaxTotalBytes int64
}

type Entry struct {
	Path string
	Data []byte
}

func AnalyzeBundle(jobID string, bundle []byte, options Options) (analyzer.Report, error) {
	entries, err := Extract(bundle, options)
	if err != nil {
		return analyzer.Report{}, err
	}
	reports := make([]analyzer.Report, 0, len(entries))
	for index, entry := range entries {
		report, err := analyzer.Analyze(fmt.Sprintf("%s-%03d", jobID, index+1), entry.Data)
		if err != nil {
			return analyzer.Report{}, fmt.Errorf("analyze bundle entry %d: %w", index+1, err)
		}
		reports = append(reports, report)
	}
	return analyzer.AggregateReports(jobID, reports, len(bundle))
}

func Extract(bundle []byte, options Options) ([]Entry, error) {
	options = normalizeOptions(options)
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return nil, fmt.Errorf("invalid gzip bundle: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var entries []Entry
	var total int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid tar bundle: %w", err)
		}
		if header == nil {
			continue
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
		name, err := safeJSONLPath(header.Name)
		if err != nil {
			return nil, err
		}
		if len(entries) >= options.MaxFiles {
			return nil, fmt.Errorf("paid scan bundle exceeds %d jsonl files", options.MaxFiles)
		}
		if header.Size < 0 || header.Size > options.MaxFileBytes {
			return nil, fmt.Errorf("paid scan file exceeds %d bytes", options.MaxFileBytes)
		}
		if total+header.Size > options.MaxTotalBytes {
			return nil, fmt.Errorf("paid scan bundle exceeds %d uncompressed bytes", options.MaxTotalBytes)
		}
		data, err := analyzer.ReadAllLimited(tarReader, options.MaxFileBytes)
		if err != nil {
			return nil, fmt.Errorf("read archive entry %s: %w", name, err)
		}
		if int64(len(data)) != header.Size {
			return nil, fmt.Errorf("archive entry size mismatch for %s", name)
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return nil, fmt.Errorf("empty jsonl file in paid scan bundle: %s", name)
		}
		total += int64(len(data))
		entries = append(entries, Entry{Path: name, Data: data})
	}
	if len(entries) == 0 {
		return nil, errors.New("paid scan bundle contains no jsonl files")
	}
	return entries, nil
}

func normalizeOptions(options Options) Options {
	if options.MaxFiles <= 0 || options.MaxFiles > DefaultMaxFiles {
		options.MaxFiles = DefaultMaxFiles
	}
	if options.MaxFileBytes <= 0 {
		options.MaxFileBytes = DefaultMaxFileBytes
	}
	if options.MaxTotalBytes <= 0 {
		options.MaxTotalBytes = DefaultMaxTotalBytes
	}
	return options
}

func safeJSONLPath(name string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) || strings.Contains(name, `\`) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean != name || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".jsonl") {
		return "", fmt.Errorf("paid scan bundle contains non-jsonl file: %s", clean)
	}
	return clean, nil
}
