package config

const (
	//? These paths must match the paths in the embed directive

	StaticLocalDir = "static"
	StaticUrlPath  = "/" + StaticLocalDir + "/"

	PostsLocalDir = "posts"
	PostsUrlPath  = "/" + PostsLocalDir + "/"

	TemplatesLocalDir = "templates"
)
