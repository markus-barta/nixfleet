package dashboard

import "context"

type contextKey string

const sessionContextKey contextKey = "session"

// withSession adds a session to the context.
func withSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// sessionFromContext retrieves the session from the context.
func sessionFromContext(ctx context.Context) *Session {
	session, _ := ctx.Value(sessionContextKey).(*Session)
	return session
}

