package mixed

import "fmt"

type Greeter struct{ Prefix string }

func (g Greeter) Hello(name string) string {
	return fmt.Sprintf("%s %s", g.Prefix, name)
}

func Run() string {
	return Greeter{Prefix: "hello"}.Hello("agent")
}
