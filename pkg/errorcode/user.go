package errorcode

import "google.golang.org/grpc/codes"

// User service codes, 2000-2999.
const (
	UserInvalidUsername  Code = 2001
	UserInvalidEmail     Code = 2002
	UserInvalidPassword  Code = 2003
	UserEmailTaken       Code = 2004
	UserInvalidLogin     Code = 2005
	UserBadCredential    Code = 2006
	UserDisabled         Code = 2007
	UserInvalidToken     Code = 2008
	UserNotFound         Code = 2009
	UserInvalidUserID    Code = 2010
	UserPasswordMismatch Code = 2011
	UserInvalidAvatar    Code = 2012
	UserNothingToUpdate  Code = 2013
	UserInvalidProfile   Code = 2014
	UserAccountTaken     Code = 2015
)

var (
	ErrUserInvalidUsername  = Define(UserInvalidUsername, codes.InvalidArgument, "username is required")
	ErrUserInvalidEmail     = Define(UserInvalidEmail, codes.InvalidArgument, "email is invalid")
	ErrUserInvalidPassword  = Define(UserInvalidPassword, codes.InvalidArgument, "password length must be 6 to 72 bytes")
	ErrUserEmailTaken       = Define(UserEmailTaken, codes.AlreadyExists, "email already registered")
	ErrUserInvalidLogin     = Define(UserInvalidLogin, codes.InvalidArgument, "email, password and platform_id are required")
	ErrUserBadCredential    = Define(UserBadCredential, codes.Unauthenticated, "email or password is incorrect")
	ErrUserDisabled         = Define(UserDisabled, codes.PermissionDenied, "user is disabled")
	ErrUserInvalidToken     = Define(UserInvalidToken, codes.Unauthenticated, "invalid token")
	ErrUserNotFound         = Define(UserNotFound, codes.NotFound, "user not found")
	ErrUserInvalidUserID    = Define(UserInvalidUserID, codes.InvalidArgument, "user_id is invalid")
	ErrUserPasswordMismatch = Define(UserPasswordMismatch, codes.Unauthenticated, "old password is incorrect")
	ErrUserInvalidAvatar    = Define(UserInvalidAvatar, codes.InvalidArgument, "avatar_url is too long")
	ErrUserNothingToUpdate  = Define(UserNothingToUpdate, codes.InvalidArgument, "nothing to update")
	ErrUserInvalidProfile   = Define(UserInvalidProfile, codes.InvalidArgument, "profile field is invalid")
	ErrUserAccountTaken     = Define(UserAccountTaken, codes.AlreadyExists, "phone or account already registered")
)
