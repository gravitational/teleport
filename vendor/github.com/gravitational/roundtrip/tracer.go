package roundtrip

import (
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
)

func NewTracer() *LogTracer {
	return &LogTracer{}
}

func (t *LogTracer) Start(r *http.Request) {
	t.StartTime = time.Now().UTC()
	t.Request.URL = r.URL.String()
	t.Request.Method = r.Method
}

func (t *LogTracer) Done(re *Response, err error) (*Response, error) {
	t.EndTime = time.Now().UTC()
	if err != nil {
		log.Infof("[TRACE] %v %v %v -> ERR: %v", t.EndTime.Sub(t.StartTime), t.Request.Method, t.Request.URL, err)
		return re, err
	}
	log.Infof("[TRACE] %v %v %v -> STATUS %v", t.EndTime.Sub(t.StartTime), t.Request.Method, t.Request.URL, re.Code)
	return re, err
}

type LogTracer struct {
	StartTime      time.Time
	EndTime        time.Time
	Request        RequestInfo
	ResponseStatus string
	ResponseError  error
}

type RequestInfo struct {
	Method string `json:"method"` // Method - request method
	URL    string `json:"url"`    // URL - Request URL
}
