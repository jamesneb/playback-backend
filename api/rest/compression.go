package rest

import (
	"compress/gzip"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// compressionMiddleware provides gzip compression
func compressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client supports gzip
		if !strings.Contains(c.Request.Header.Get(string(HEADER_ACCEPT_ENCODING)), ENCODING_GZIP) {
			c.Next()
			return
		}

		// Skip compression for certain content types
		contentType := c.Request.Header.Get(string(HEADER_CONTENT_TYPE))
		if strings.Contains(contentType, CONTENT_TYPE_IMAGE_PREFIX) || strings.Contains(contentType, CONTENT_TYPE_VIDEO_PREFIX) {
			c.Next()
			return
		}

		// Wrap response writer with gzip writer
		gzipWriter := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			level:          DEFAULT_COMPRESSION_LEVEL,
			minSize:        COMPRESSION_MIN_SIZE,
			maxBufferSize:  MAX_COMPRESSION_BUFFER_SIZE,
			headerSet:      false,
		}

		originalWriter := c.Writer
		c.Writer = gzipWriter
		c.Header(string(HEADER_VARY), string(HEADER_ACCEPT_ENCODING))

		defer func() {
			if err := gzipWriter.finalize(); err != nil {
				logger.Error("Compression finalization failed", zap.Error(err))
				// Restore original writer and try to write uncompressed
				c.Writer = originalWriter
			}
		}()

		c.Next()
	}
}

// gzipResponseWriter wraps gin.ResponseWriter with gzip compression
type gzipResponseWriter struct {
	gin.ResponseWriter
	gzipWriter    *gzip.Writer
	level         int
	minSize       int
	maxBufferSize int
	buffer        []byte
	headerSet     bool
	totalWritten  int
	compressionEnabled bool
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	dataLen := len(data)

	// Buffer small responses up to limits
	if w.gzipWriter == nil && !w.compressionEnabled {
		// Prevent buffer overflow
		if len(w.buffer)+dataLen > w.maxBufferSize {
			// Buffer too large, write uncompressed
			if len(w.buffer) > 0 {
				if _, err := w.ResponseWriter.Write(w.buffer); err != nil {
					return 0, err
				}
				w.buffer = nil
			}
			written, err := w.ResponseWriter.Write(data)
			w.totalWritten += written
			return written, err
		}

		// Add to buffer
		w.buffer = append(w.buffer, data...)

		// Check if we should start compression
		if len(w.buffer) >= w.minSize {
			if err := w.initializeCompression(); err != nil {
				// Compression failed, fallback to uncompressed
				logger.Warn("Compression initialization failed, using uncompressed", zap.Error(err))
				written, err := w.ResponseWriter.Write(w.buffer)
				w.totalWritten += written
				w.buffer = nil
				return dataLen, err // Return requested length, not actual written
			}
		} else {
			w.totalWritten += dataLen
			return dataLen, nil // Successfully buffered
		}
	}

	// Write via gzip if initialized
	if w.gzipWriter != nil {
		written, err := w.gzipWriter.Write(data)
		w.totalWritten += written
		return dataLen, err // Always return original data length for HTTP semantics
	}

	// Direct write (uncompressed)
	written, err := w.ResponseWriter.Write(data)
	w.totalWritten += written
	return written, err
}

func (w *gzipResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// initializeCompression starts gzip compression
func (w *gzipResponseWriter) initializeCompression() error {
	var err error
	w.gzipWriter, err = gzip.NewWriterLevel(w.ResponseWriter, w.level)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}

	// Set compression header
	w.Header().Set(string(HEADER_CONTENT_ENCODING), ENCODING_GZIP)
	w.headerSet = true
	w.compressionEnabled = true

	// Write buffered data
	if len(w.buffer) > 0 {
		if _, err := w.gzipWriter.Write(w.buffer); err != nil {
			if closeErr := w.gzipWriter.Close(); closeErr != nil {
				logger.Error("Failed to close gzip writer after write error", zap.Error(closeErr))
			}
			w.gzipWriter = nil
			return fmt.Errorf("failed to write buffered data: %w", err)
		}
		w.buffer = nil
	}

	return nil
}

// finalize completes compression or writes uncompressed data
func (w *gzipResponseWriter) finalize() error {
	if w.gzipWriter != nil {
		if err := w.gzipWriter.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
		return nil
	}

	// Write any remaining buffered data uncompressed
	if len(w.buffer) > 0 {
		if _, err := w.ResponseWriter.Write(w.buffer); err != nil {
			return fmt.Errorf("failed to write uncompressed buffer: %w", err)
		}
		w.buffer = nil
	}

	return nil
}