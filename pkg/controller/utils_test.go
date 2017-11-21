package controller

type dummyLogger struct{}

func (d dummyLogger) Log(keyvals ...interface{}) error {
	return nil
}
