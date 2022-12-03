package statsig

type IDataAdapter interface {
	get(key string) string
	set(key string, value string)
	initialize()
	shutdown()
}
