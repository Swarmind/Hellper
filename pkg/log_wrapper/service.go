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

	// Omit filename, since it gives us the full path, which is non-desired
	// The FuncForPC already gives us a package name, so filename most likely will be tracked
	// TODO room for improvement
	pc, _, no, ok := runtime.Caller(depth)
	if ok {
		f := runtime.FuncForPC(pc)
		errFmt = fmt.Sprintf("%s:%d: %v", f.Name(), no, err)
	}

	s.Log.Println(errFmt)

	return errFmt
}
