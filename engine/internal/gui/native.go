package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/yetone/magpie/internal/library"
)

func nativeError(rw http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, library.ErrNativeConflict):
		status = http.StatusConflict
	case errors.Is(err, fs.ErrNotExist):
		status = http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		status = http.StatusForbidden
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
}

func nativeDecode(rw http.ResponseWriter, r *http.Request, in any) bool {
	r.Body = http.MaxBytesReader(rw, r.Body, 8<<20)
	if err := json.NewDecoder(r.Body).Decode(in); err != nil {
		nativeError(rw, errors.New("invalid native resource request"))
		return false
	}
	return true
}

func nativeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/native", func(rw http.ResponseWriter, r *http.Request) {
		if root := r.URL.Query().Get("root"); root != "" {
			offset, limit := 0, 0
			fmt.Sscan(r.URL.Query().Get("offset"), &offset)
			fmt.Sscan(r.URL.Query().Get("limit"), &limit)
			page, err := library.ReadNativeRoot(root, offset, limit)
			if err != nil {
				nativeError(rw, err)
				return
			}
			writeJSON(rw, page)
			return
		}
		writeJSON(rw, library.ReadNative())
	})
	mux.HandleFunc("GET /api/native/file", func(rw http.ResponseWriter, r *http.Request) {
		file, err := library.ReadNativeFile(r.URL.Query().Get("id"))
		if err != nil {
			nativeError(rw, err)
			return
		}
		writeJSON(rw, file)
	})
	mux.HandleFunc("POST /api/native/file", func(rw http.ResponseWriter, r *http.Request) {
		var in library.NativeChange
		if !nativeDecode(rw, r, &in) {
			return
		}
		if err := library.ChangeNativeFile(in); err != nil {
			nativeError(rw, err)
			return
		}
		writeJSON(rw, library.ReadNative())
	})
	mux.HandleFunc("POST /api/native/copy/preview", func(rw http.ResponseWriter, r *http.Request) {
		var in library.NativeCopyRequest
		if !nativeDecode(rw, r, &in) {
			return
		}
		preview, err := library.PreviewNativeCopy(in)
		if err != nil {
			nativeError(rw, err)
			return
		}
		writeJSON(rw, preview)
	})
	mux.HandleFunc("POST /api/native/copy/apply", func(rw http.ResponseWriter, r *http.Request) {
		var in library.NativeCopyApply
		if !nativeDecode(rw, r, &in) {
			return
		}
		result, err := library.ApplyNativeCopy(in)
		if err != nil {
			nativeError(rw, err)
			return
		}
		writeJSON(rw, result)
	})
}
