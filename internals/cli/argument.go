package cli

import (
	"net/url"
)

type Argument struct {
	Value       ArgValue
	Name        string
	Required    bool
	Placeholder string
	Description string
	Hidden      bool
}

type ArgValue interface {
	Set(string) error
}

func ArgumentRegister(params []Argument, args []string) error {
	for i, arg := range args {
		err := params[i].Value.Set(arg)
		if err != nil {
			return err
		}
	}
	return nil
}

func ArgumentArrRegister(params []Argument, args []string) error {
	for _, arg := range args {
		err := params[0].Value.Set(arg)
		if err != nil {
			return err
		}
	}
	return nil
}

type StringValue struct {
	Param string
}

func (s *StringValue) Set(replacer string) error {
	s.Param = replacer
	return nil
}

type StringArrValue struct {
	Param []string
}

func (s *StringArrValue) Set(replacer string) error {
	s.Param = append(s.Param, replacer)
	return nil
}

type URLValue struct {
	*url.URL
}

func (s *URLValue) Set(replacer string) error {
	var err error
	s.URL, err = url.Parse(replacer)
	return err
}

type ByteValue struct {
	Param []byte
}

func (s *ByteValue) Set(replacer string) error {
	s.Param = []byte(replacer)
	return nil
}

// Registerer allows others to register commands on it.
type Registerer interface {
	Command(cmd string, help string) *CommandClause
}

func getRequired(params []Argument) int {
	required := 0
	for _, arg := range params {
		if arg.Required {
			required++
		}
	}
	return required
}