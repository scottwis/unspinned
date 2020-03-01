package readers

import "io"

type unionReader struct {
	readers []io.Reader
}

func (u *unionReader) Read(p []byte) (int, error) {

	for {
		if len(u.readers) == 0 {
			return 0, io.EOF
		}

		n, err := u.readers[0].Read(p)

		if err == io.EOF {
			u.readers = u.readers[1:]
			if len(u.readers) != 0 {
				err = nil
			}
		}

		if !(n == 0 && err == nil && len(p) != 0) {
			return n, err
		}
	}
}

func Union(readers... io.Reader) io.Reader {
	var ret unionReader

	for _, r := range readers {
		ur, ok := r.(*unionReader)

		if ok {
			ret.readers = append(ret.readers, ur.readers...)
		} else {
			ret.readers = append(ret.readers, r)
		}
	}

	return &ret
}

