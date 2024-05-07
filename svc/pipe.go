package svc

type streamable interface {
	Write([]byte) (int, error)
	Read([]byte) (int, error)
}

func toChan(s streamable) chan []byte {
	c := make(chan []byte)

	go func() {
		b := make([]byte, 1024)

		for {
			n, err := s.Read(b)
			if n > 0 {
				res := make([]byte, n)
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}
		}
	}()
	return c
}

func pipe(s1 streamable, s2 streamable) {
	chan1 := toChan(s1)
	chan2 := toChan(s2)

	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return
			} else {
				s2.Write(b1)
			}
		case b2 := <-chan2:
			if b2 == nil {
				return
			} else {
				s1.Write(b2)
			}
		}
	}
}
