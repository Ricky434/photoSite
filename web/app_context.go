package web

type contextKey string

const isAuthenticatedContextKey = contextKey("isAuthenticated")
const userLevelContextKey = contextKey("userLevel")
