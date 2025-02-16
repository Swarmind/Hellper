package logwrapper

import (
	"fmt"
	"log"
	"runtime"
)

type Service struct {
	Log *log.Logger
}

func (s *Service) LogFormatError(err error, depth int) string {
	errFmt := err.Error()

	pc, _, no, ok := runtime.Caller(depth)
	if ok {
		f := runtime.FuncForPC(pc)
		errFmt = fmt.Sprintf("%s:%d: %v", f.Name(), no, err)
	}

	s.Log.Println(errFmt)

	return errFmt
}
