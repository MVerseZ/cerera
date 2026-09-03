package service

type Service struct {
	id      int
	Name    string
	Methods map[string]RPCHandler
}

func NewService(name string, methods map[string]RPCHandler) *Service {
	return &Service{
		Name:    name,
		Methods: methods,
	}
}

func (s *Service) ServiceName() string { return s.Name }
func (s *Service) GetID() int          { return s.id }
