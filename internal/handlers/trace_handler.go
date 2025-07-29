package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type TraceHandler struct{}

func NewTraceHandler() *TraceHandler {
	return &TraceHandler{}
}

// CreateTrace creates a new trace
// @Summary Create trace
// @Description Create a new distributed trace
// @Tags traces
// @Accept json
// @Produce json
// @Param trace body CreateTraceRequest true "Trace data"
// @Success 201 {object} TraceResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/traces [post]
func (h *TraceHandler) CreateTrace(c *gin.Context) {
	var req CreateTraceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	response := TraceResponse{
		ID:        "1",
		TraceID:   req.TraceID,
		CreatedAt: time.Now(),
	}

	c.JSON(http.StatusCreated, response)
}

// GetTrace retrieves a trace by ID
// @Summary Get trace
// @Description Get a trace by its ID
// @Tags traces
// @Produce json
// @Param id path string true "Trace ID"
// @Success 200 {object} TraceResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/traces/{id} [get]
func (h *TraceHandler) GetTrace(c *gin.Context) {
	id := c.Param("id")
	
	response := TraceResponse{
		ID:        id,
		TraceID:   "sample-trace-" + id,
		CreatedAt: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

type CreateTraceRequest struct {
	TraceID   string            `json:"trace_id" example:"abc123" binding:"required"`
	SpanID    string            `json:"span_id" example:"def456" binding:"required"`
	Timestamp time.Time         `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Tags      map[string]string `json:"tags" example:"service:api,version:1.0"`
}

type TraceResponse struct {
	ID        string    `json:"id" example:"1"`
	TraceID   string    `json:"trace_id" example:"abc123"`
	CreatedAt time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
}

type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request"`
	Message string `json:"message" example:"Field validation failed"`
}