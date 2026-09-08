package ir

type Strings struct {
	values []string
	index  map[string]int
}

func NewStrings() *Strings { return &Strings{index: map[string]int{}} }

func (s *Strings) Intern(v string) int {
	if v == "" {
		return 0
	}
	if i, ok := s.index[v]; ok {
		return i + 1
	}
	s.values = append(s.values, v)
	i := len(s.values) - 1
	s.index[v] = i
	return i + 1 // zero reserved for empty
}

func (s *Strings) Values() []string { return append([]string(nil), s.values...) }
