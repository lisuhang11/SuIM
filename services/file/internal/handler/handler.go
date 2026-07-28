package handler

import (
	pb "SuIM/proto/filepb"
	"context"
	"fileservice/internal/middleware"
	"fileservice/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedFileServiceServer
	svc *service.Service
}

func New(s *service.Service) *Handler { return &Handler{svc: s} }
func check(ctx context.Context, claimed string) error {
	if claimed == "" || claimed != middleware.UserID(ctx) {
		return status.Error(codes.PermissionDenied, "user identity mismatch")
	}
	return nil
}
func grpcErr(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch service.ErrorKind(err) {
	case "invalid":
		code = codes.InvalidArgument
	case "resource":
		code = codes.ResourceExhausted
	case "failed":
		code = codes.FailedPrecondition
	case "permission":
		code = codes.PermissionDenied
	case "not_found":
		code = codes.NotFound
	}
	return status.Error(code, err.Error())
}
func (h *Handler) InitiateUpload(ctx context.Context, r *pb.InitiateUploadReq) (*pb.InitiateUploadResp, error) {
	if err := check(ctx, r.UserId); err != nil {
		return nil, err
	}
	v, err := h.svc.Initiate(ctx, r.UserId, r.Name, r.ContentType, r.Sha256, r.Size)
	return v, grpcErr(err)
}
func (h *Handler) CompleteUpload(ctx context.Context, r *pb.CompleteUploadReq) (*pb.CompleteUploadResp, error) {
	if err := check(ctx, r.UserId); err != nil {
		return nil, err
	}
	f, err := h.svc.Complete(ctx, r.UserId, r.FileId)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &pb.CompleteUploadResp{File: service.ToProto(f)}, nil
}
func (h *Handler) BindFile(ctx context.Context, r *pb.BindFileReq) (*pb.BindFileResp, error) {
	if err := check(ctx, r.UserId); err != nil {
		return nil, err
	}
	f, err := h.svc.Bind(ctx, r.UserId, r.FileId, r.ConversationId)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &pb.BindFileResp{File: service.ToProto(f)}, nil
}
func (h *Handler) GetDownloadURL(ctx context.Context, r *pb.GetDownloadURLReq) (*pb.GetDownloadURLResp, error) {
	if err := check(ctx, r.UserId); err != nil {
		return nil, err
	}
	f, u, expiry, err := h.svc.Download(ctx, r.UserId, r.FileId)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &pb.GetDownloadURLResp{File: service.ToProto(f), DownloadUrl: u, ExpiresAt: expiry.UnixMilli()}, nil
}
func (h *Handler) GetFile(ctx context.Context, r *pb.GetFileReq) (*pb.GetFileResp, error) {
	if err := check(ctx, r.UserId); err != nil {
		return nil, err
	}
	f, err := h.svc.Get(ctx, r.UserId, r.FileId)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &pb.GetFileResp{File: service.ToProto(f)}, nil
}
func (h *Handler) DeleteFile(ctx context.Context, r *pb.DeleteFileReq) (*pb.DeleteFileResp, error) {
	if err := check(ctx, r.UserId); err != nil {
		return nil, err
	}
	if err := h.svc.Delete(ctx, r.UserId, r.FileId); err != nil {
		return nil, grpcErr(err)
	}
	return &pb.DeleteFileResp{}, nil
}
