package config

const (
	//? These paths must match the paths in the embed directive

	StaticLocalDir = "static"
	StaticURLPath  = "/" + StaticLocalDir + "/"

	PostsLocalDir = "posts"
	PostsURLPath  = "/" + PostsLocalDir + "/"

	TemplatesLocalDir = "templates"

	TemplateLayout = "layout.html"
	TemplateIndex  = "index.html"
	TemplatePost   = "post.html"
	TemplateEditor = "editor.html"

	// Template names (without .html extension)
	TemplateNameAuth = "ed25519_auth"
)
