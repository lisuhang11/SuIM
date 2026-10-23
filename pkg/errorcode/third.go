package errorcode

import "google.golang.org/grpc/codes"

// Third service codes, 5000-5999.
const (
	ThirdInvalidArgs  Code = 5001
	ThirdNotFound     Code = 5002
	ThirdBadSign      Code = 5003
	ThirdHashMismatch Code = 5004
	ThirdNameConflict Code = 5005
	ThirdTooLarge     Code = 5006
	ThirdPartSize     Code = 5007
	ThirdSizeMismatch Code = 5008
)

var (
	ErrThirdInvalidArgs  = Define(ThirdInvalidArgs, codes.InvalidArgument, "invalid upload argument")
	ErrThirdNotFound     = Define(ThirdNotFound, codes.NotFound, "upload not found")
	ErrThirdBadSign      = Define(ThirdBadSign, codes.Unauthenticated, "invalid part signature")
	ErrThirdHashMismatch = Define(ThirdHashMismatch, codes.InvalidArgument, "part hash mismatch")
	ErrThirdNameConflict = Define(ThirdNameConflict, codes.AlreadyExists, "object name already exists")
	ErrThirdTooLarge     = Define(ThirdTooLarge, codes.InvalidArgument, "object is too large")
	ErrThirdPartSize     = Define(ThirdPartSize, codes.InvalidArgument, "part size mismatch")
	ErrThirdSizeMismatch = Define(ThirdSizeMismatch, codes.InvalidArgument, "file size mismatch")
)
