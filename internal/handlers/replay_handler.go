package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
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

type DownloadRequest struct {
	Key string `json:"key" binding:"required"`
}

// ListReplays lists all available replay files from S3
// @Summary List replay files
// @Description List all available Arrow IPC replay files from S3
// @Tags replays
// @Produce json
// @Success 200 {array} ReplayFile
// @Router /replays/list [get]
func (h *ReplayHandler) ListReplays(c *gin.Context) {
	ctx := context.Background()
	
	// List objects in the replays bucket
	listOutput, err := h.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(h.bucketName),
		Prefix: aws.String("replays/"), // Filter to replay files
	})
	
	if err != nil {
		logger.Error("Failed to list S3 objects", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list replay files",
		})
		return
	}
	
	files := make([]ReplayFile, 0)
	for _, obj := range listOutput.Contents {
		// Extract job ID from key if possible (e.g., "replays/job-123.arrow" -> "job-123")
		jobID := ""
		if obj.Key != nil && len(*obj.Key) > 8 { // "replays/" = 8 chars
			keyPart := (*obj.Key)[8:] // Remove "replays/" prefix
			if len(keyPart) > 6 {     // Remove ".arrow" suffix
				jobID = keyPart[:len(keyPart)-6]
			}
		}
		
		file := ReplayFile{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			JobID:        jobID,
		}
		
		if obj.LastModified != nil {
			file.LastModified = obj.LastModified.Format("2006-01-02T15:04:05Z")
		}
		
		files = append(files, file)
	}
	
	// Add cache-busting headers
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	
	logger.Info("Listed replay files", zap.Int("count", len(files)))
	c.JSON(http.StatusOK, files)
}

// DownloadReplay downloads a specific replay file from S3
// @Summary Download replay file
// @Description Download a specific Arrow IPC replay file from S3
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
	
	ctx := context.Background()
	
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
	defer getOutput.Body.Close()
	
	// Read the file content
	var buf bytes.Buffer
	_, err = io.Copy(&buf, getOutput.Body)
	if err != nil {
		logger.Error("Failed to read S3 object", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read replay file",
		})
		return
	}
	
	// Set headers for binary download with cache-busting
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+req.Key)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("ETag", fmt.Sprintf("\"%d\"", time.Now().UnixNano()))
	
	logger.Info("Downloaded replay file", 
		zap.String("key", req.Key), 
		zap.Int("size", buf.Len()))
	
	// Return the binary data
	c.Data(http.StatusOK, "application/octet-stream", buf.Bytes())
}