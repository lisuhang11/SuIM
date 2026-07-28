// Package common_user 定义通知模块对用户模型的抽象接口。
// pkg/notification 不依赖任何具体 user model，只认此接口。
package common_user

// CommonUser 是通知模块对任意用户模型的抽象。
// 各业务服务通过适配器将自身 UserInfo 类型转为此接口后注入。
type CommonUser interface {
	GetNickname() string
	GetFaceURL()  string
	GetUserID()   string
}
