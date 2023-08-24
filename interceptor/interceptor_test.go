package interceptor

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

type MockNet struct {
	header http.Header
	status int
	buf    *bytes.Buffer
}

func (m *MockNet) Header() http.Header {
	return m.header
}

func (m *MockNet) Write(bs []byte) (int, error) {
	return m.buf.Write(bs)
}

func (m *MockNet) WriteHeader(statusCode int) {
	m.status = statusCode
}

func NewMockNet() http.ResponseWriter {
	return &MockNet{header: make(http.Header), buf: bytes.NewBuffer([]byte{})}
}

var (
	in = map[string]interface{}{
		"username": "test",
		"password": "abcdefg",
		"info": map[string]interface{}{
			"idNum": "111111111111111111",
		},
	}
	rm = map[string]Replace{
		"password": func(bs []byte) string {
			return "******"
		},
		"idNum": func(bs []byte) string {
			return "xxxxxx"
		},
	}
)

func TestReplace(t *testing.T) {
	recMap(in, rm)
	assert.Equal(t, "******", in["password"])
	assert.Equal(t, "xxxxxx", in["info"].(map[string]interface{})["idNum"])
}

func TestResponseWriter(t *testing.T) {
	reqBody, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(reqBody))
	if err != nil {
		panic(err)
	}
	req.RequestURI = "/api/login"

	writer := NewResponseWriter(NewMockNet(), req)
	WithEventTypeMap(writer, map[string]string{
		"POST_/api/login": "登陆",
	})
	WithKeywordMap(writer, rm)
	writer.WriteHeader(http.StatusOK)
	writer.Write(reqBody)
	time.Sleep(time.Second)
}
