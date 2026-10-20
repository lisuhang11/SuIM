package webhook

import "github.com/lisuhang11/SuIM/pkg/errorcode"

const (
	// ActionAllow lets the business action proceed.
	ActionAllow int32 = 0
	// ActionForbidden stops the business action.
	ActionForbidden int32 = 1
	// NextCodeContinue asks the caller to keep going after this callback.
	NextCodeContinue int32 = 1
)

// CallbackReq is the JSON body posted to a webhook command.
type CallbackReq interface {
	GetCallbackCommand() string
}

// CallbackResp is the JSON body returned by a webhook. Parse reports how the caller should proceed.
type CallbackResp interface {
	Parse() error
}

// CommonCallbackReq is the shared header of a callback request.
type CommonCallbackReq struct {
	CallbackCommand string `json:"callbackCommand"`
	OperationID     string `json:"operationID"`
}

func (c CommonCallbackReq) GetCallbackCommand() string {
	return c.CallbackCommand
}

// CommonCallbackResp is the shared callback response.
type CommonCallbackResp struct {
	ActionCode int32  `json:"actionCode"`
	ErrCode    int32  `json:"errCode"`
	ErrMsg     string `json:"errMsg"`
	ErrDlt     string `json:"errDlt"`
	NextCode   int32  `json:"nextCode"`
}

func (c *CommonCallbackResp) Parse() error {
	if c == nil {
		return errorcode.ErrWebhookData.WithMsg("nil callback response")
	}
	if c.ActionCode == ActionForbidden {
		msg := c.ErrMsg
		if msg == "" {
			msg = "callback rejected"
		}
		if c.ErrDlt != "" {
			msg += ": " + c.ErrDlt
		}
		return errorcode.ErrWebhookRejected.WithMsg(msg)
	}
	if c.NextCode == NextCodeContinue {
		return errorcode.ErrWebhookContinue
	}
	return nil
}
