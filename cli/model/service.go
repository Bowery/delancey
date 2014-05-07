package model

import "net/http"

type Service struct {
	Address  string            `address`
	Commands map[string]string `commands`
}

func (s *Service) Ping() error {
	res, err := http.Get(s.Address)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
