package piagent

import (
	"net/http"
	"reflect"
)

var _flusherType = reflect.TypeOf((*http.Flusher)(nil)).Elem()

// flushCompatibleWriter 在底层 writer 未实现 http.Flusher 时,
// 通过 flushRegardless 反射穿透注入的包装层完成 Flush,
// 以保证 SSE 流式推送在包装器(如 SkyWalking auto-instrumentation)下仍可用。
type flushCompatibleWriter struct {
	http.ResponseWriter
}

func (f flushCompatibleWriter) Flush() {
	flushRegardless(f.ResponseWriter)
}

func flushRegardless(w http.ResponseWriter) bool {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
		return true
	}
	v := reflect.ValueOf(w)
	if target := findFlusher(v, 0); target.IsValid() {
		target.MethodByName("Flush").Call(nil)
		return true
	}
	if h, ok := w.(http.Hijacker); ok {
		if conn, bw, err := h.Hijack(); err == nil {
			_ = conn
			if bw != nil {
				bw.Flush()
			}
			return true
		}
	}
	return false
}

func findFlusher(v reflect.Value, depth int) reflect.Value {
	if depth > 12 {
		return reflect.Value{}
	}
	if v.Type().Implements(_flusherType) && v.CanInterface() {
		return v
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if !v.IsNil() {
			if r := findFlusher(v.Elem(), depth+1); r.IsValid() {
				return r
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fv := v.Field(i)
			if !fv.CanInterface() {
				if v.CanAddr() {
					fv = v.Addr().Elem().Field(i)
				}
				if !fv.CanInterface() {
					continue
				}
			}
			if r := findFlusher(fv, depth+1); r.IsValid() {
				return r
			}
		}
	}
	return reflect.Value{}
}