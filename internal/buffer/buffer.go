// YSM - Yandere SQL Manager
// Copyright (C) 2025 blubskye
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Source code: https://github.com/blubskye/yandere_sql_manager

package buffer

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/blubskye/yandere_sql_manager/internal/logging"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Default buffer sizes for different operations
const (
	// SmallBufferSize for small files or quick operations (64KB)
	SmallBufferSize = 64 * 1024

	// DefaultBufferSize for typical database operations (1MB)
	DefaultBufferSize = 1024 * 1024

	// LargeBufferSize for large database operations (8MB)
	LargeBufferSize = 8 * 1024 * 1024

	// HugeBufferSize for very large databases (32MB)
	HugeBufferSize = 32 * 1024 * 1024

	// SQLStatementBufferSize for reading SQL statements (256KB)
	SQLStatementBufferSize = 256 * 1024
)

// CompressionType represents supported compression formats
type CompressionType string

const (
	CompressionNone CompressionType = ""
	CompressionGzip CompressionType = "gzip"
	CompressionXZ   CompressionType = "xz"
	CompressionZstd CompressionType = "zstd"
)

// DetectCompression detects compression type from filename
func DetectCompression(filename string) CompressionType {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".gzip"):
		return CompressionGzip
	case strings.HasSuffix(lower, ".xz"):
		return CompressionXZ
	case strings.HasSuffix(lower, ".zst") || strings.HasSuffix(lower, ".zstd"):
		return CompressionZstd
	default:
		return CompressionNone
	}
}

// BufferedReader wraps an io.Reader with buffering and optional decompression
type BufferedReader struct {
	file       *os.File
	decompressor io.ReadCloser
	reader     *bufio.Reader
	bufferSize int
}

// NewBufferedReader creates a new buffered reader with optional decompression
func NewBufferedReader(path string, bufferSize int) (*BufferedReader, error) {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}

	logging.Debug("Opening file for reading: %s (buffer: %d bytes)", path, bufferSize)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	br := &BufferedReader{
		file:       file,
		bufferSize: bufferSize,
	}

	// Detect and apply decompression
	compression := DetectCompression(path)
	var reader io.Reader = file

	switch compression {
	case CompressionGzip:
		logging.Debug("Detected gzip compression")
		gzr, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		br.decompressor = gzr
		reader = gzr

	case CompressionXZ:
		logging.Debug("Detected xz compression")
		xzr, err := xz.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		// xz.Reader doesn't implement io.Closer, wrap it
		br.decompressor = io.NopCloser(xzr)
		reader = xzr

	case CompressionZstd:
		logging.Debug("Detected zstd compression")
		zstdr, err := zstd.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create zstd reader: %w", err)
		}
		br.decompressor = zstdr.IOReadCloser()
		reader = br.decompressor

	default:
		logging.Debug("No compression detected")
	}

	br.reader = bufio.NewReaderSize(reader, bufferSize)
	return br, nil
}

// Read implements io.Reader
func (br *BufferedReader) Read(p []byte) (n int, err error) {
	return br.reader.Read(p)
}

// ReadLine reads a line from the buffer
func (br *BufferedReader) ReadLine() (string, error) {
	line, err := br.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), err
}

// ReadBytes reads bytes until the delimiter
func (br *BufferedReader) ReadBytes(delim byte) ([]byte, error) {
	return br.reader.ReadBytes(delim)
}

// Close closes all underlying readers
func (br *BufferedReader) Close() error {
	var errs []error

	if br.decompressor != nil {
		if err := br.decompressor.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := br.file.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// BufferedWriter wraps an io.Writer with buffering and optional compression
type BufferedWriter struct {
	file       *os.File
	compressor io.WriteCloser
	writer     *bufio.Writer
	bufferSize int
}

// NewBufferedWriter creates a new buffered writer with optional compression
func NewBufferedWriter(path string, compression CompressionType, bufferSize int) (*BufferedWriter, error) {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}

	logging.Debug("Opening file for writing: %s (buffer: %d bytes, compression: %s)", path, bufferSize, compression)

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	bw := &BufferedWriter{
		file:       file,
		bufferSize: bufferSize,
	}

	var writer io.Writer = file

	switch compression {
	case CompressionGzip:
		logging.Debug("Using gzip compression")
		gzw, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		bw.compressor = gzw
		writer = gzw

	case CompressionXZ:
		logging.Debug("Using xz compression")
		xzw, err := xz.NewWriter(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create xz writer: %w", err)
		}
		bw.compressor = xzw
		writer = xzw

	case CompressionZstd:
		logging.Debug("Using zstd compression")
		zstdw, err := zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create zstd writer: %w", err)
		}
		bw.compressor = zstdw
		writer = zstdw

	default:
		logging.Debug("No compression")
	}

	bw.writer = bufio.NewWriterSize(writer, bufferSize)
	return bw, nil
}

// Write implements io.Writer
func (bw *BufferedWriter) Write(p []byte) (n int, err error) {
	return bw.writer.Write(p)
}

// WriteString writes a string to the buffer
func (bw *BufferedWriter) WriteString(s string) (n int, err error) {
	return bw.writer.WriteString(s)
}

// Flush flushes the buffer
func (bw *BufferedWriter) Flush() error {
	return bw.writer.Flush()
}

// Close flushes and closes all underlying writers
func (bw *BufferedWriter) Close() error {
	var errs []error

	if err := bw.writer.Flush(); err != nil {
		errs = append(errs, fmt.Errorf("flush error: %w", err))
	}

	if bw.compressor != nil {
		if err := bw.compressor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("compressor close error: %w", err))
		}
	}

	if err := bw.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("file close error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// SQLStatementReader reads SQL statements from a buffered source
// It handles multi-line statements and respects string literals
type SQLStatementReader struct {
	reader     *BufferedReader
	buffer     strings.Builder
	delimiter  string
	lineNumber int
}

// NewSQLStatementReader creates a new SQL statement reader
func NewSQLStatementReader(path string) (*SQLStatementReader, error) {
	reader, err := NewBufferedReader(path, SQLStatementBufferSize)
	if err != nil {
		return nil, err
	}

	return &SQLStatementReader{
		reader:    reader,
		delimiter: ";",
	}, nil
}

// SetDelimiter sets the statement delimiter
func (sr *SQLStatementReader) SetDelimiter(d string) {
	sr.delimiter = d
}

// ReadStatement reads the next complete SQL statement
// Returns the statement, line number where it started, and any error
func (sr *SQLStatementReader) ReadStatement() (string, int, error) {
	sr.buffer.Reset()
	startLine := sr.lineNumber + 1
	inString := false
	stringChar := byte(0)
	escaped := false

	for {
		line, err := sr.reader.ReadLine()
		sr.lineNumber++

		if err == io.EOF && sr.buffer.Len() == 0 {
			return "", 0, io.EOF
		}

		// Skip empty lines and comments at start
		trimmed := strings.TrimSpace(line)
		if sr.buffer.Len() == 0 {
			if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "#") {
				if err == io.EOF {
					return "", 0, io.EOF
				}
				continue
			}
			startLine = sr.lineNumber
		}

		// Check for DELIMITER command
		if sr.buffer.Len() == 0 && strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			newDelim := strings.TrimSpace(trimmed[10:])
			if newDelim != "" {
				sr.delimiter = newDelim
				logging.Debug("SQL delimiter changed to: %s", newDelim)
			}
			if err == io.EOF {
				return "", 0, io.EOF
			}
			continue
		}

		// Add line to buffer
		if sr.buffer.Len() > 0 {
			sr.buffer.WriteByte('\n')
		}
		sr.buffer.WriteString(line)

		// Check if statement is complete
		// Need to track string state to not match delimiter inside strings
		for i := 0; i < len(line); i++ {
			c := line[i]

			if escaped {
				escaped = false
				continue
			}

			if c == '\\' {
				escaped = true
				continue
			}

			if inString {
				if c == stringChar {
					inString = false
				}
				continue
			}

			if c == '\'' || c == '"' || c == '`' {
				inString = true
				stringChar = c
				continue
			}
		}

		// Check for delimiter at end of line (only if not in string)
		if !inString {
			content := sr.buffer.String()
			trimmedContent := strings.TrimSpace(content)
			if strings.HasSuffix(trimmedContent, sr.delimiter) {
				// Remove delimiter from result
				result := strings.TrimSuffix(trimmedContent, sr.delimiter)
				return strings.TrimSpace(result), startLine, nil
			}
		}

		if err == io.EOF {
			// Return whatever we have
			content := strings.TrimSpace(sr.buffer.String())
			if content != "" {
				return content, startLine, nil
			}
			return "", 0, io.EOF
		}
	}
}

// Close closes the underlying reader
func (sr *SQLStatementReader) Close() error {
	return sr.reader.Close()
}

// LineNumber returns the current line number
func (sr *SQLStatementReader) LineNumber() int {
	return sr.lineNumber
}

// BatchWriter buffers writes and flushes in batches
type BatchWriter struct {
	writer      *BufferedWriter
	batchSize   int
	currentSize int
	mu          sync.Mutex
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(path string, compression CompressionType, batchSize int) (*BatchWriter, error) {
	writer, err := NewBufferedWriter(path, compression, LargeBufferSize)
	if err != nil {
		return nil, err
	}

	if batchSize <= 0 {
		batchSize = DefaultBufferSize
	}

	return &BatchWriter{
		writer:    writer,
		batchSize: batchSize,
	}, nil
}

// Write writes data and flushes if batch size exceeded
func (bw *BatchWriter) Write(p []byte) (n int, err error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	n, err = bw.writer.Write(p)
	bw.currentSize += n

	if bw.currentSize >= bw.batchSize {
		if flushErr := bw.writer.Flush(); flushErr != nil {
			return n, flushErr
		}
		bw.currentSize = 0
	}

	return n, err
}

// WriteString writes a string
func (bw *BatchWriter) WriteString(s string) (n int, err error) {
	return bw.Write([]byte(s))
}

// Flush forces a flush
func (bw *BatchWriter) Flush() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	bw.currentSize = 0
	return bw.writer.Flush()
}

// Close flushes and closes
func (bw *BatchWriter) Close() error {
	return bw.writer.Close()
}

// ProgressReader wraps a reader and tracks progress
type ProgressReader struct {
	reader     io.Reader
	totalSize  int64
	readBytes  int64
	onProgress func(read, total int64)
	mu         sync.Mutex
}

// NewProgressReader creates a progress-tracking reader
func NewProgressReader(reader io.Reader, totalSize int64, onProgress func(read, total int64)) *ProgressReader {
	return &ProgressReader{
		reader:     reader,
		totalSize:  totalSize,
		onProgress: onProgress,
	}
}

// Read implements io.Reader with progress tracking
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)

	pr.mu.Lock()
	pr.readBytes += int64(n)
	read := pr.readBytes
	pr.mu.Unlock()

	if pr.onProgress != nil && n > 0 {
		pr.onProgress(read, pr.totalSize)
	}

	return n, err
}

// Progress returns current progress
func (pr *ProgressReader) Progress() (read, total int64) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.readBytes, pr.totalSize
}

// ProgressWriter wraps a writer and tracks progress
type ProgressWriter struct {
	writer       io.Writer
	writtenBytes int64
	onProgress   func(written int64)
	mu           sync.Mutex
}

// NewProgressWriter creates a progress-tracking writer
func NewProgressWriter(writer io.Writer, onProgress func(written int64)) *ProgressWriter {
	return &ProgressWriter{
		writer:     writer,
		onProgress: onProgress,
	}
}

// Write implements io.Writer with progress tracking
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)

	pw.mu.Lock()
	pw.writtenBytes += int64(n)
	written := pw.writtenBytes
	pw.mu.Unlock()

	if pw.onProgress != nil && n > 0 {
		pw.onProgress(written)
	}

	return n, err
}

// Written returns bytes written
func (pw *ProgressWriter) Written() int64 {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	return pw.writtenBytes
}

// RecommendedBufferSize returns a recommended buffer size based on file size
func RecommendedBufferSize(fileSize int64) int {
	switch {
	case fileSize < 1024*1024: // < 1MB
		return SmallBufferSize
	case fileSize < 100*1024*1024: // < 100MB
		return DefaultBufferSize
	case fileSize < 1024*1024*1024: // < 1GB
		return LargeBufferSize
	default: // >= 1GB
		return HugeBufferSize
	}
}

// GetFileSize returns the size of a file
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
