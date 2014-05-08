package pubsub

type NetWriter struct{}

func NewNetWriter() *NetWriter {
	return &NetWriter{}
}

func (nw *NetWriter) Write(d []byte) (int, error) {
	if err := Publish(d); err != nil {
		return 0, err
	}

	return len(d), nil
}
