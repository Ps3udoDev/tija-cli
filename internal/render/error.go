package render

import (
	"encoding/json"
	"io"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSONError writes {"error":{"code":code,"message":"..."}} followed by newline.
func JSONError(w io.Writer, code int, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorBody{
			Code:    code,
			Message: msg,
		},
	})
}
