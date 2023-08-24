package interceptor

var (
	defaultKeyword map[string]Replace
)

type Replace func([]byte) string

func init() {
	defaultKeyword = map[string]Replace{}
}

func recMap(in map[string]interface{}, kw map[string]Replace) {
	for k, v := range in {
		sv, ok := v.(string)
		if ok {
			rep, ok := kw[k]
			if ok {
				in[k] = rep([]byte(sv))
			}
			continue
		}

		mv, ok := v.(map[string]interface{})
		if ok {
			recMap(mv, kw)
		}
	}
}
