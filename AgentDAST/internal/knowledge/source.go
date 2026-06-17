package knowledge

type documentSource interface {
	get(base string) (string, bool)
	list() []string
}

var currentSource documentSource = emptySource{}

type emptySource struct{}

func (emptySource) get(string) (string, bool) { return "", false }

func (emptySource) list() []string { return nil }

func init() {
	_ = Init("")
}
