package interceptor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/2908755265/http-middleware/constm"
	"github.com/2908755265/mutil/ctxm"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	MLogKey = "M-Log-Key"
)

var (
	EventType          field = "EventType"
	Username           field = "Username"
	IP                 field = "IP"
	Content            field = "Content"
	Result             field = "Result"
	defaultIpHeaderKey       = "X-Real-Ip"
)

type field string

type LogInfo struct {
	EventType string    `json:"eventType"`
	Username  string    `json:"username"`
	Ip        string    `json:"ip"`
	Content   string    `json:"content"`
	Result    int       `json:"result"`
	CreatedAt time.Time `json:"createdAt"`
}

type LogHandler func(info *LogInfo) error
type ResultAssert func(map[string]interface{}) bool

type loggerResponseWriter struct {
	logx.Logger
	origin       http.ResponseWriter
	status       int
	req          *http.Request
	reqBody      []byte
	rspBody      []byte
	fromHeader   *logInfo
	eventTypeMap map[string]string
	keyword      map[string]Replace
	reqTime      time.Time
	lh           LogHandler
	ipHeaderKey  string
	rspMap       map[string]interface{}
	resultAssert func(map[string]interface{}) bool
}

type logInfo struct {
	eventType string
	username  string
	ip        string
	content   string
	result    int
}

func (w *loggerResponseWriter) Header() http.Header {
	return w.origin.Header()
}

func (w *loggerResponseWriter) Write(bs []byte) (int, error) {
	w.rspBody = bs
	// 日志打印
	w.log()
	return w.origin.Write(bs)
}

func (w *loggerResponseWriter) WriteHeader(statusCode int) {
	// 设置
	w.status = statusCode
	w.origin.WriteHeader(statusCode)
}

func (w *loggerResponseWriter) log() {
	// 异步日志
	threading.GoSafe(func() {
		lp := w.buildLogParam()
		// 未构建日志参数，无需记录
		if lp == nil {
			return
		}

		err := w.lh(lp)
		if err != nil {
			w.Error(err)
			return
		}
		w.Debug("日志处理完成")
	})
}

func (w *loggerResponseWriter) buildLogParam() *LogInfo {
	w.parseFromHeader()
	// 解析后，移除特定header
	defer w.origin.Header().Del(MLogKey)

	// 如果未配置事件类型，表示无需记录该日志
	et := w.getEventType()
	if et == "" {
		return nil
	}

	if len(w.rspBody) > 0 {
		w.parseRspToMap()
	}

	// 解析特定header
	ol := &LogInfo{
		EventType: et,
		Username:  w.getUsername(),
		Ip:        w.getIP(),
		Content:   w.getContent(),
		Result:    w.getResult(),
		CreatedAt: w.reqTime,
	}

	return ol
}

func (w *loggerResponseWriter) getUsername() string {
	if w.fromHeader != nil && w.fromHeader.username != "" {
		return w.fromHeader.username
	}
	ui := ctxm.GetUserInfo(w.req.Context())
	if ui != nil {
		return ui.Username
	}
	return ""
}

func (w *loggerResponseWriter) getIP() string {
	if w.fromHeader != nil && w.fromHeader.ip != "" {
		return w.fromHeader.ip
	}
	return w.req.Header.Get(w.ipHeaderKey)
}

func (w *loggerResponseWriter) getEventType() string {
	if w.fromHeader != nil && w.fromHeader.eventType != "" {
		return w.fromHeader.eventType
	}
	return w.eventTypeMap[fmt.Sprintf("%s_%s", w.req.Method, w.req.RequestURI)]
}

func (w *loggerResponseWriter) getContent() string {
	var req []byte
	var rsp []byte

	if len(w.reqBody) > 0 {
		reqMap := make(map[string]interface{})
		err := json.Unmarshal(w.reqBody, &reqMap)
		if err != nil {
			w.Error(err)
		} else {
			req = w.replaceKeyword(reqMap)
		}
	}

	if w.fromHeader != nil && w.fromHeader.content != "" {
		rsp = []byte(w.fromHeader.content)
	} else {
		if w.rspMap != nil {
			rsp = w.replaceKeyword(w.rspMap)
		}
	}

	return fmt.Sprintf("请求参数 : %s\n响应结果 : %s", string(req), string(rsp))
}

func (w *loggerResponseWriter) replaceKeyword(v map[string]interface{}) []byte {
	recMap(v, w.keyword)
	rsp, err := json.Marshal(v)
	if err != nil {
		w.Error(err)
	}
	return rsp
}

func (w *loggerResponseWriter) parseRspToMap() {
	v := make(map[string]interface{})
	err := json.Unmarshal(w.rspBody, &v)
	if err != nil {
		w.Error(err)
	}
	w.rspMap = v
}

func (w *loggerResponseWriter) getResult() int {
	if w.fromHeader != nil && w.fromHeader.result > 0 {
		return w.fromHeader.result
	}
	if w.status >= http.StatusBadRequest {
		return constm.OpLogResultFailed
	} else {
		return w.parseResultFromBody()
	}
}

func (w *loggerResponseWriter) parseResultFromBody() int {
	if w.rspMap == nil {
		return constm.OpLogResultFailed
	}

	if w.resultAssert(w.rspMap) {
		return constm.OpLogResultSuccess
	}

	return constm.OpLogResultFailed
}

func (w *loggerResponseWriter) parseFromHeader() {
	values := w.origin.Header().Values(MLogKey)
	if len(values) != 0 {
		fh := new(logInfo)
		for _, value := range values {
			w.setLogInfo(fh, []byte(value))
		}
		w.fromHeader = fh
	}
}

func (w *loggerResponseWriter) setLogInfo(li *logInfo, value []byte) {
	key := make([]byte, 0)
	var val []byte
	for i, b := range value {
		if b != '=' {
			key = append(key, b)
		} else {
			val = value[i+1:]
			break
		}
	}
	switch field(key) {
	case EventType:
		li.eventType = string(val)
	case Username:
		li.username = string(val)
	case IP:
		li.ip = string(val)
	case Content:
		li.content = string(val)
	case Result:
		result, _ := strconv.ParseInt(string(val), 10, 64)
		li.result = int(result)
	}
}

func defaultResultAssert(r map[string]interface{}) bool {
	code, ok := r["code"]
	if ok {
		fc, ok := code.(float64)
		if ok {
			return int(fc) == 0
		}
	}
	return false
}

func NewResponseWriter(ori http.ResponseWriter, r *http.Request) http.ResponseWriter {
	buf := bytes.NewBuffer([]byte{})
	_, _ = io.Copy(buf, r.Body)
	body := buf.Bytes()
	r.Body = ioutil.NopCloser(buf)
	rw := &loggerResponseWriter{
		Logger:       logx.WithContext(r.Context()),
		origin:       ori,
		lh:           defaultLogHandler,
		req:          r,
		reqBody:      body,
		eventTypeMap: defaultEventTypeMap,
		keyword:      defaultKeyword,
		reqTime:      time.Now(),
		ipHeaderKey:  defaultIpHeaderKey,
		resultAssert: defaultResultAssert,
	}
	return rw
}

func WithEventTypeMap(w http.ResponseWriter, etm map[string]string) {
	lrw := w.(*loggerResponseWriter)
	lrw.eventTypeMap = etm
}

func WithKeywordMap(w http.ResponseWriter, km map[string]Replace) {
	lrw := w.(*loggerResponseWriter)
	lrw.keyword = km
}

func WithLogHandler(w http.ResponseWriter, h LogHandler) {
	lrw := w.(*loggerResponseWriter)
	lrw.lh = h
}

func WithIpHeaderKey(w http.ResponseWriter, key string) {
	lrw := w.(*loggerResponseWriter)
	lrw.ipHeaderKey = key
}

func WithResultAssert(w http.ResponseWriter, ra ResultAssert) {
	lrw := w.(*loggerResponseWriter)
	lrw.resultAssert = ra
}

func AddLogKV(w http.ResponseWriter, key field, val string) {
	lrw, ok := w.(*loggerResponseWriter)
	if ok {
		lrw.origin.Header().Add(MLogKey, fmt.Sprintf("%s=%s", key, val))
	} else {
		logx.Error("set KV failed: not a loggerResponseWriter")
	}
}
