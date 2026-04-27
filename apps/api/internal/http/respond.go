package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Meta is the pagination envelope for list responses.
type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
	Total   int `json:"total"`
}

// errorBody mirrors the spec: { code, message }.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK writes a single-resource success response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data, "error": nil})
}

// Created writes a 201 with the same envelope.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data, "error": nil})
}

// NoContent writes a 204 (no envelope).
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// List writes a list success response with pagination meta.
func List(c *gin.Context, data any, meta Meta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta, "error": nil})
}

// Err writes an error response with the given HTTP status, error code, and message.
func Err(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"data":  nil,
		"error": errorBody{Code: code, Message: message},
	})
}
