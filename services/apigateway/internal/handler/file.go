package handler

import (
	"context"
	"net/http"
	"time"

	pb "SuIM/proto/filepb"
	"github.com/gin-gonic/gin"
)

type FileHandler struct{ client pb.FileServiceClient }

func NewFileHandler(client pb.FileServiceClient) *FileHandler { return &FileHandler{client: client} }
func (h *FileHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/initiate", h.Initiate)
	r.POST("/:id/complete", h.Complete)
	r.GET("/:id", h.Get)
	r.GET("/:id/download", h.Download)
	r.GET("/:id/avatar", h.Download)
	r.DELETE("/:id", h.Delete)
}
func (h *FileHandler) Initiate(c *gin.Context) {
	var body struct {
		Name        string `json:"name" binding:"required"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size" binding:"required"`
		SHA256      string `json:"sha256"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.InitiateUpload(ctx, &pb.InitiateUploadReq{UserId: userIDFromCtx(c), Name: body.Name, ContentType: body.ContentType, Size: body.Size, Sha256: body.SHA256})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
func (h *FileHandler) Complete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 90*time.Second)
	defer cancel()
	resp, err := h.client.CompleteUpload(ctx, &pb.CompleteUploadReq{UserId: userIDFromCtx(c), FileId: c.Param("id")})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *FileHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	resp, err := h.client.GetFile(ctx, &pb.GetFileReq{UserId: userIDFromCtx(c), FileId: c.Param("id")})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
func (h *FileHandler) Download(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	resp, err := h.client.GetDownloadURL(ctx, &pb.GetDownloadURLReq{UserId: userIDFromCtx(c), FileId: c.Param("id")})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
func (h *FileHandler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.DeleteFile(ctx, &pb.DeleteFileReq{UserId: userIDFromCtx(c), FileId: c.Param("id")})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
