package ctxutil

import "context"

type ctxKey string

const userIDKey ctxKey = "userID"

// WithUserID 将用户ID存入context
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID 从context获取用户ID
func GetUserID(ctx context.Context) int64 {
	if v, ok := ctx.Value(userIDKey).(int64); ok {
		return v
	}
	return 0
}
