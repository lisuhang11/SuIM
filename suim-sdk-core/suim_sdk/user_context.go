package suim_sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"SuIM/suim-sdk-core/internal/user"
	"SuIM/suim-sdk-core/pkg/api"
	"SuIM/suim-sdk-core/pkg/ccontext"
	"SuIM/suim-sdk-core/pkg/db"
	"SuIM/suim-sdk-core/pkg/sdkerrs"
	"SuIM/suim-sdk-core/sdk_struct"
	"SuIM/suim-sdk-core/suim_sdk_callback"
)

const (
	LogoutStatus = iota + 1
	Logging
	Logged
)

var (
	IMUserContext *UserContext
	once          sync.Once
)

func init() {
	once.Do(func() {
		IMUserContext = NewIMUserContext()
	})
}

// UserContext mirrors OpenIM's global runtime.
type UserContext struct {
	user         *user.User
	db           *db.DataBase
	loginStatus  int
	loginUserID  string
	info         *ccontext.GlobalConfig
	ctx          context.Context
	cancel       context.CancelFunc
	userListener suim_sdk_callback.OnUserListener
	connListener suim_sdk_callback.OnConnListener
	mu           sync.Mutex
}

func NewIMUserContext() *UserContext {
	u := &UserContext{
		info: &ccontext.GlobalConfig{},
		user: user.NewUser(),
	}
	u.ctx, u.cancel = context.WithCancel(ccontext.WithInfo(context.Background(), u.info))
	u.user.SetListener(u.UserListener)
	return u
}

func (u *UserContext) User() *user.User                 { return u.user }
func (u *UserContext) Info() *ccontext.GlobalConfig     { return u.info }
func (u *UserContext) Context() context.Context         { return u.ctx }

func (u *UserContext) UserListener() suim_sdk_callback.OnUserListener {
	if u.userListener == nil {
		return emptyUserListener{}
	}
	return u.userListener
}

func (u *UserContext) SetUserListener(listener suim_sdk_callback.OnUserListener) {
	u.userListener = listener
}

func (u *UserContext) setLoginStatus(s int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.loginStatus = s
}

func (u *UserContext) getLoginStatus() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.loginStatus
}

type emptyUserListener struct{}

func (emptyUserListener) OnSelfInfoUpdated(string)   {}
func (emptyUserListener) OnUserStatusChanged(string) {}

func CheckResourceLoad(userContext *UserContext, funcName string) error {
	if userContext == nil || userContext.Info() == nil || userContext.Info().ApiAddr == "" {
		return sdkerrs.ErrNotInit
	}
	short := funcName
	if parts := strings.Split(funcName, "."); len(parts) > 0 {
		short = parts[len(parts)-1]
	}
	switch short {
	case "Login", "InitSDK", "login", "loginWithPassword", "LoginWithPassword":
		return nil
	}
	if strings.Contains(short, "Login") || strings.Contains(short, "login") {
		return nil
	}
	if userContext.getLoginStatus() != Logged {
		return sdkerrs.ErrNotLogin
	}
	return nil
}

func call(callback suim_sdk_callback.Base, operationID string, fn any, args ...any) {
	if callback == nil {
		return
	}
	go func() {
		res, err := call_(operationID, fn, args...)
		if err != nil {
			if code, ok := err.(sdkerrs.CodeError); ok {
				callback.OnError(int32(code.Code()), err.Error())
			} else {
				callback.OnError(int32(sdkerrs.UnknownCode), err.Error())
			}
			return
		}
		if res == nil {
			callback.OnSuccess("")
			return
		}
		switch v := res.(type) {
		case string:
			callback.OnSuccess(v)
			return
		}
		data, err := json.Marshal(res)
		if err != nil {
			callback.OnError(int32(sdkerrs.SdkInternalError), err.Error())
			return
		}
		callback.OnSuccess(string(data))
	}()
}

func call_(operationID string, fn any, args ...any) (any, error) {
	fnv := reflect.ValueOf(fn)
	funcName := runtime.FuncForPC(fnv.Pointer()).Name()
	if err := CheckResourceLoad(IMUserContext, funcName); err != nil {
		return nil, err
	}
	fnt := fnv.Type()
	ctx := ccontext.WithOperationID(IMUserContext.Context(), operationID)
	ins := []reflect.Value{reflect.ValueOf(ctx)}
	for i := 0; i < len(args); i++ {
		tag := fnt.In(i + 1)
		argVal := args[i]
		argType := reflect.TypeOf(argVal)
		if argType != nil && (argType.AssignableTo(tag) || tag.Kind() == reflect.Interface) {
			ins = append(ins, reflect.ValueOf(argVal))
			continue
		}
		if argType != nil && argType.Kind() == reflect.String {
			s := argVal.(string)
			switch tag.Kind() {
			case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map, reflect.Ptr:
				v := reflect.New(tag)
				if s != "" {
					if err := json.Unmarshal([]byte(s), v.Interface()); err != nil {
						return nil, sdkerrs.Wrap(sdkerrs.ArgsError, "json unmarshal arg", err)
					}
				}
				ins = append(ins, v.Elem())
				continue
			case reflect.String:
				ins = append(ins, reflect.ValueOf(s))
				continue
			}
		}
		return nil, fmt.Errorf("arg type mismatch at %d: want %s got %v", i, tag, argType)
	}
	outs := fnv.Call(ins)
	if len(outs) == 0 {
		return nil, nil
	}
	last := outs[len(outs)-1]
	errType := reflect.TypeOf((*error)(nil)).Elem()
	if last.IsValid() && last.Type().Implements(errType) {
		if !last.IsNil() {
			return nil, last.Interface().(error)
		}
	}
	if len(outs) == 1 {
		return nil, nil
	}
	return outs[0].Interface(), nil
}

func listenerCall(fn any, listener any) {
	if IMUserContext == nil {
		return
	}
	reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(listener)})
}

// InitSDK initializes API address and data dir.
func InitSDK(config *sdk_struct.IMConfig, listener suim_sdk_callback.OnConnListener) bool {
	if config == nil || config.ApiAddr == "" {
		return false
	}
	IMUserContext.info.ApiAddr = strings.TrimRight(config.ApiAddr, "/")
	IMUserContext.info.DataDir = config.DataDir
	if IMUserContext.info.DataDir == "" {
		IMUserContext.info.DataDir = "./suim_data"
	}
	IMUserContext.connListener = listener
	IMUserContext.ctx, IMUserContext.cancel = context.WithCancel(ccontext.WithInfo(context.Background(), IMUserContext.info))
	return true
}

// Login binds userID + token (OpenIM pattern).
func Login(callback suim_sdk_callback.Base, operationID, userID, token string) {
	call(callback, operationID, IMUserContext.login, userID, token)
}

func (u *UserContext) login(ctx context.Context, userID, token string) error {
	if u.getLoginStatus() == Logged {
		return sdkerrs.New(sdkerrs.ArgsError, "already logged in")
	}
	u.setLoginStatus(Logging)
	u.info.UserID = userID
	u.info.Token = token
	u.loginUserID = userID

	database, err := db.NewDataBase(u.info.DataDir, userID)
	if err != nil {
		u.setLoginStatus(LogoutStatus)
		return err
	}
	u.db = database
	u.user.SetDataBase(database)
	u.user.SetLoginUserID(userID)
	u.setLoginStatus(Logged)
	_ = u.user.SyncLoginUserInfo(ctx)
	return nil
}

// LoginWithPassword calls SuIM /users/login then Login(userID, token).
func LoginWithPassword(callback suim_sdk_callback.Base, operationID, email, password string) {
	call(callback, operationID, IMUserContext.loginWithPassword, email, password)
}

func (u *UserContext) loginWithPassword(ctx context.Context, email, password string) (map[string]any, error) {
	resp, err := api.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.User == nil || resp.AccessToken == "" {
		return nil, sdkerrs.New(sdkerrs.SdkInternalError, "empty login response")
	}
	if err := u.login(ctx, resp.User.UserID, resp.AccessToken); err != nil {
		return nil, err
	}
	return map[string]any{
		"userID":       resp.User.UserID,
		"accessToken":  resp.AccessToken,
		"refreshToken": resp.RefreshToken,
		"user":         resp.User,
	}, nil
}

// Logout clears login state.
func Logout(callback suim_sdk_callback.Base, operationID string) {
	call(callback, operationID, IMUserContext.logout)
}

func (u *UserContext) logout(ctx context.Context) error {
	_ = ctx
	u.setLoginStatus(LogoutStatus)
	u.info.Token = ""
	u.loginUserID = ""
	return nil
}
