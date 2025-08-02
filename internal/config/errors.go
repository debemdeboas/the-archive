package config

const (
	// Database errors
	ErrInitializeDatabaseFmt = "Failed to initialize database: %v"
	ErrGetPostsFmt          = "Failed to get posts: %v"
	
	// Auth errors  
	ErrCreateProviderFmt    = "Failed to create provider: %v"
	ErrAuthHeaderRequired   = "Authorization header required"
	ErrInvalidSignatureFormat = "Invalid signature format" 
	ErrInvalidSignature     = "Invalid signature"
	ErrInternalServerError  = "Internal server error"
	
	// Config errors
	ErrWriteConfigContentFmt = "Failed to write config content: %v"
	ErrCreateTempFileFmt    = "Failed to create temp file: %v"
	
	// Post processing errors
	ErrInitializingPosts = "Error initializing posts"
	ErrReloadingPosts   = "Error reloading posts"
	
	// Challenge errors
	ErrRefreshChallengeFmt = "Failed to refresh challenge"
)