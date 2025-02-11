package cache

var staticCache = NewCache[string, string]()

func GetStaticHash(path string) (string, bool) {
	return staticCache.Get(path)
}

func SetStaticHash(path, hash string) {
	staticCache.Set(path, hash)
}
