package common

type sections struct {
	contains map[string]*section
}

func (ss sections) section(name string) *section {
	s, ok := ss.contains[name]
	if !ok {
		s = new(section)
		ss.contains[name] = s
	}
	return s
}

type section struct {
	content map[string]*value
}

func (s *section) key(name string) *value {
	v, ok := s.content[name]
	if !ok {
		v = new(value)
		s.content[name] = v
	}
	return v
}
