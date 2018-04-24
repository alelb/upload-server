package message

// Error is
type Error struct {
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

// NewError is
func NewError(code int, detail string) Error {
	return Error{
		Code:   code,
		Detail: detail,
	}
}
