package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Pagination limits for S3 operations to prevent DoS attacks
	DefaultMaxKeys = 100
	MaxAllowedKeys = 1000

	// Default prefix for replay files
	DefaultReplayPrefix = "tenants/default/replay-deltas/"

	// HTTP headers
	HeaderContentType        = "Content-Type"
	HeaderContentDisposition = "Content-Disposition"
	HeaderCacheControl       = "Cache-Control"
	HeaderPragma             = "Pragma"
	HeaderExpires            = "Expires"
	HeaderETag               = "ETag"
	HeaderContentLength      = "Content-Length"

	// MIME types
	MimeOctetStream = "application/octet-stream"

	// Cache control values
	NoCacheValue = "no-cache, no-store, must-revalidate"
	PragmaNoCache = "no-cache"
	ExpiresZero  = "0"

	// Path validation
	PathTraversalPattern = ".."
	DoubleSlashPattern   = "//"
)

type ReplayHandler struct {
	s3Client   *s3.Client
	bucketName string
}

func NewReplayHandler(s3Client *s3.Client, bucketName string) *ReplayHandler {
	return &ReplayHandler{
		s3Client:   s3Client,
		bucketName: bucketName,
	}
}

type ReplayFile struct {
	Key          string `json:"key"`
	LastModified string `json:"lastModified"`
	Size         int64  `json:"size"`
	JobID        string `json:"jobId,omitempty"`
}

type ListReplaysResponse struct {
	Files           []ReplayFile `json:"files"`
	IsTruncated     bool         `json:"isTruncated"`
	NextToken       *string      `json:"nextToken,omitempty"`
	TotalCount      int          `json:"totalCount"`
	MaxKeys         int          `json:"maxKeys"`
}

type DownloadRequest struct {
	Key string `json:"key" binding:"required"`
}

// ListReplays lists available replay files from S3 with pagination
// @Summary List replay files
// @Description List available Arrow IPC replay files from S3 with pagination
// @Tags replays
// @Produce json
// @Param limit query int false "Maximum number of files to return (default: 100, max: 1000)"
// @Param token query string false "Continuation token for pagination"
// @Param prefix query string false "Prefix to filter files"
// @Success 200 {object} ListReplaysResponse
// @Router /replays/list [get]
func (h *ReplayHandler) ListReplays(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination parameters with security limits
	maxKeys := DefaultMaxKeys
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			if parsed > 0 && parsed <= MaxAllowedKeys {
				maxKeys = parsed
			} else if parsed > MaxAllowedKeys {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("limit cannot exceed %d", MaxAllowedKeys),
				})
				return
			}
		}
	}

	token := c.Query("token")
	prefix := c.DefaultQuery("prefix", DefaultReplayPrefix)

	logger.Info("Starting to list replay files",
		zap.String("bucket", h.bucketName),
		zap.String("prefix", prefix),
		zap.Int("maxKeys", maxKeys),
		zap.String("token", token))

	// Build S3 request with pagination and security limits
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(h.bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(maxKeys)),
	}

	if token != "" {
		input.ContinuationToken = aws.String(token)
	}

	listOutput, err := h.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		logger.Error("Failed to list S3 objects", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list replay files",
		})
		return
	}

	logger.Info("S3 ListObjectsV2 succeeded",
		zap.Int("object_count", len(listOutput.Contents)),
		zap.Bool("is_truncated", listOutput.IsTruncated != nil && *listOutput.IsTruncated))

	files := make([]ReplayFile, 0, len(listOutput.Contents))
	for _, obj := range listOutput.Contents {
		// Extract job ID from key (e.g., "tenants/default/replay-deltas/abc123/timestamp.arrow" -> "abc123")
		jobID := ""
		if obj.Key != nil {
			key := *obj.Key
			// Remove the prefix
			if strings.HasPrefix(key, prefix) {
				remaining := key[len(prefix):] // Remove prefix
				// Find the next slash to get the job ID
				slashIndex := strings.Index(remaining, "/")
				if slashIndex > 0 {
					jobID = remaining[:slashIndex]
				}
			}
		}

		file := ReplayFile{
			Key:   aws.ToString(obj.Key),
			Size:  aws.ToInt64(obj.Size),
			JobID: jobID,
		}

		if obj.LastModified != nil {
			file.LastModified = obj.LastModified.Format("2006-01-02T15:04:05Z")
		}

		files = append(files, file)
	}

	// Build paginated response
	response := ListReplaysResponse{
		Files:       files,
		IsTruncated: listOutput.IsTruncated != nil && *listOutput.IsTruncated,
		TotalCount:  len(files),
		MaxKeys:     maxKeys,
	}

	if response.IsTruncated && listOutput.NextContinuationToken != nil {
		response.NextToken = listOutput.NextContinuationToken
	}

	// Add cache-busting headers
	c.Header(HeaderCacheControl, NoCacheValue)
	c.Header(HeaderPragma, PragmaNoCache)
	c.Header(HeaderExpires, ExpiresZero)

	logger.Info("Listed replay files",
		zap.Int("count", len(files)),
		zap.Bool("truncated", response.IsTruncated),
		zap.Int("maxKeys", maxKeys))

	c.JSON(http.StatusOK, response)
}

// DownloadReplay streams a specific replay file from S3
// @Summary Download replay file
// @Description Stream a specific Arrow IPC replay file directly from S3
// @Tags replays
// @Accept json
// @Produce application/octet-stream
// @Param request body DownloadRequest true "Download request"
// @Success 200 {file} binary
// @Router /replays/download [post]
func (h *ReplayHandler) DownloadReplay(c *gin.Context) {
	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Validate key to prevent path traversal attacks
	if strings.Contains(req.Key, PathTraversalPattern) || strings.Contains(req.Key, DoubleSlashPattern) {
		logger.Warn("Blocked potential path traversal attempt",
			zap.String("key", req.Key))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid key format",
		})
		return
	}

	logger.Info("Starting replay file download",
		zap.String("key", req.Key),
		zap.String("bucket", h.bucketName))

	// Get object from S3
	getOutput, err := h.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(h.bucketName),
		Key:    aws.String(req.Key),
	})

	if err != nil {
		logger.Error("Failed to get S3 object", zap.Error(err), zap.String("key", req.Key))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Replay file not found",
		})
		return
	}
	defer func() {
		if err := getOutput.Body.Close(); err != nil {
			logger.Error("Failed to close S3 response body", zap.Error(err))
		}
	}()

	// Set headers for streaming binary download
	c.Header(HeaderContentType, MimeOctetStream)
	c.Header(HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", req.Key))
	c.Header(HeaderCacheControl, NoCacheValue)
	c.Header(HeaderPragma, PragmaNoCache)
	c.Header(HeaderExpires, ExpiresZero)

	// Use S3's ETag if available for better caching behavior
	if getOutput.ETag != nil {
		c.Header(HeaderETag, *getOutput.ETag)
	}

	// Set content length if available for better client experience
	if getOutput.ContentLength != nil {
		c.Header(HeaderContentLength, fmt.Sprintf("%d", *getOutput.ContentLength))
	}

	logger.Info("Streaming replay file",
		zap.String("key", req.Key),
		zap.Int64("contentLength", aws.ToInt64(getOutput.ContentLength)))

	// Stream directly from S3 to client without buffering in memory
	// This prevents OOM attacks and reduces latency
	bytesWritten, err := io.Copy(c.Writer, getOutput.Body)
	if err != nil {
		logger.Error("Failed to stream S3 object",
			zap.Error(err),
			zap.String("key", req.Key),
			zap.Int64("bytesWritten", bytesWritten))
		// Cannot send JSON error after streaming has started
		return
	}

	logger.Info("Successfully streamed replay file",
		zap.String("key", req.Key),
		zap.Int64("bytesWritten", bytesWritten))
}
