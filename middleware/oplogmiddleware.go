package middleware

import (
	"github.com/2908755265/http-middleware/interceptor"
	"net/http"
)

type OpLogMiddleware struct {
	h     interceptor.LogHandler
	etm   map[string]string
	rep   map[string]interceptor.Replace
	ipKey string
	ra    interceptor.ResultAssert
}

func NewOpLogMiddleware(h interceptor.LogHandler, etm map[string]string, replace map[string]interceptor.Replace, ra interceptor.ResultAssert, ipKey string) *OpLogMiddleware {
	return &OpLogMiddleware{h: h, etm: etm, rep: replace, ra: ra, ipKey: ipKey}
}

func (m *OpLogMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writer := interceptor.NewResponseWriter(w, r)

		if m.etm != nil {
			interceptor.WithEventTypeMap(writer, m.etm)
		}
		if m.rep != nil {
			interceptor.WithKeywordMap(writer, m.rep)
		}
		if m.h != nil {
			interceptor.WithLogHandler(writer, m.h)
		}
		if m.ipKey != "" {
			interceptor.WithIpHeaderKey(writer, m.ipKey)
		}
		if m.ra != nil {
			interceptor.WithResultAssert(writer, m.ra)
		}

		next(writer, r)
	}
}
