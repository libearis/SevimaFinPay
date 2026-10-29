package response

import "github.com/gin-gonic/gin"

// SuccessEnvelope represents a standardized successful JSON response.
type SuccessEnvelope struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

// ErrorDetail holds machine-readable error code and human-readable message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope represents a standardized error JSON response.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// OK sends a 200 OK JSON response with the standard envelope.
func OK(c *gin.Context, data interface{}) {
	c.JSON(200, SuccessEnvelope{
		Status: "ok",
		Data:   data,
	})
}

// Created sends a 201 Created JSON response with the standard envelope.
func Created(c *gin.Context, data interface{}) {
	c.JSON(201, SuccessEnvelope{
		Status: "ok",
		Data:   data,
	})
}

// Error sends an error JSON response with the given HTTP status code.
func Error(c *gin.Context, httpStatus int, code string, message string) {
	c.JSON(httpStatus, ErrorEnvelope{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
