package tmpl

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

type valueType interface {
	executer
	Value(*context) (interface{}, error)
}

// ***********
// * Helpers *
// ***********

func isConstantValue(v valueType) bool {
	switch v.(type) {
	case intValue, floatValue, constantValue:
		return true
	}
	return false
}

func writeValue(w io.Writer, v interface{}) (err error) {
	switch c := v.(type) {
	case []byte:
		_, err = w.Write(c)
	default:
		_, err = fmt.Fprintf(w, "%v", c)
	}
	return
}

// *******************
// * Parsing Helpers *
// *******************

func isValueType(tok token) bool {
	switch tok.typ {
	case tokenStartSel, tokenCall, tokenValue, tokenNumeric:
		return true
	}
	return false
}

func isBasicValueType(tok token) bool {
	switch tok.typ {
	case tokenStartSel, tokenValue, tokenNumeric:
		return true
	}
	return false
}

func numericToValue(tok token) (v valueType, err error) {
	if tok.typ != tokenNumeric {
		return nil, fmt.Errorf("expected numeric got %q", tok)
	}
	sval := string(tok.dat)
	i, err := strconv.ParseInt(sval, 10, 64)
	if err == nil {
		return intValue(i), nil
	}
	f, err := strconv.ParseFloat(sval, 64)
	if err == nil {
		return floatValue(f), nil
	}
	return
}

func consumeValue(p *parser) (valueType, error) {
	switch tok := p.next(); tok.typ {
	case tokenStartSel, tokenValue, tokenNumeric:
		p.backup()
		return consumeBasicValue(p)
	case tokenCall:
		return consumeCallValue(p)
	default:
		return nil, fmt.Errorf("Expected a value type got a %q", tok)
	}
	return nil, nil
}

func consumeBasicValue(p *parser) (valueType, error) {
	switch tok := p.next(); tok.typ {
	case tokenStartSel:
		p.backup()
		return consumeSelector(p)
	case tokenValue:
		return constantValue(tok.dat), nil
	case tokenNumeric:
		return numericToValue(tok)
	default:
		return nil, fmt.Errorf("Expected a value type got got a %q", tok)
	}
	return nil, nil
}

// ******************
// * Selector Value *
// ******************

type selectorValue struct {
	pops int
	abs  bool
	path []string
}

func (s *selectorValue) Value(c *context) (v interface{}, err error) {
	rv, err := c.valueFor(s)
	if err != nil {
		return
	}
	v = rv.Interface()
	return
}

func (s *selectorValue) Execute(w io.Writer, c *context) (err error) {
	val, err := s.Value(c)
	if err != nil {
		return
	}
	err = writeValue(w, val)
	return
}

func (s *selectorValue) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[selector $%d", s.pops)
	for _, tok := range s.path {
		fmt.Fprintf(&buf, " %s", tok)
	}
	fmt.Fprint(&buf, "]")
	return buf.String()
}

func consumeSelector(p *parser) (val *selectorValue, err error) {
	if tok := p.next(); tok.typ != tokenStartSel {
		return nil, fmt.Errorf("Expected a %q got a %q", tokenStartSel, tok)
	}

	//at this point the tokenStartSel should be consumed
	val, err = consumeSelectorHeader(p)
	if err != nil {
		return
	}

	//consume a push selector
	if tok := p.next(); tok.typ != tokenPush {
		return nil, fmt.Errorf("Unexpected %q. Expected a %q", tok, tokenPush)
	}

	//check the first special case of an empty push
	switch next := p.next(); next.typ {
	case tokenEndSel:
		return
	case tokenIdent:
		//we got a pair so thats part of our path
		val.path = append(val.path, string(next.dat))
	default:
		return nil, fmt.Errorf("Unexpected %q. Expected a %q or %q.", next, tokenEndSel, tokenIdent)
	}

	for {
		switch tok := p.next(); tok.typ {
		case tokenEndSel:
			return
		case tokenPush:
		default:
			return nil, fmt.Errorf("Expected a %q, got a %q", tokenPush, tok)
		}

		tok := p.next()
		if tok.typ != tokenIdent {
			return nil, fmt.Errorf("Expected a %q, got a %q", tokenIdent, tok)
		}
		val.path = append(val.path, string(tok.dat))
	}

	panic("unreachable")
}

func consumeSelectorHeader(p *parser) (val *selectorValue, err error) {
	switch tok := p.next(); tok.typ {
	case tokenRoot:
		return &selectorValue{0, true, nil}, nil
	case tokenPush:
		p.backup()
		return &selectorValue{}, nil
	case tokenPop:
		var pops int
		for pops = 1; p.next().typ == tokenPop; pops++ {
		}
		p.backup()
		return &selectorValue{pops, false, nil}, nil
	default:
		return nil, fmt.Errorf("Unexpected %q. Expected a %q, %q, or %q", tok, tokenRoot, tokenPush, tokenPop)
	}

	panic("unreachable")
}

// **************
// * Call Value *
// **************

type callValue struct {
	name []byte
	args []valueType
}

func (s callValue) Value(c *context) (v interface{}, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("call %s: %v", s.name, e)
		}
	}()

	fnc := c.getCall(string(s.name))
	var (
		params []reflect.Value
		val    interface{}
	)
	for _, arg := range s.args {
		val, err = arg.Value(c)
		if err != nil {
			return
		}
		params = append(params, reflect.ValueOf(val))
	}

	switch res := fnc.Call(params); len(res) {
	case 0:
	case 1:
		v = res[0].Interface()
	default:
		var ret []interface{}
		for _, item := range res {
			ret = append(ret, item.Interface())
		}
		v = ret
	}

	return
}

func (s callValue) Execute(w io.Writer, c *context) (err error) {
	val, err := s.Value(c)
	if err != nil {
		return
	}
	err = writeValue(w, val)
	return
}

func (s callValue) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[call %s", string(s.name))
	for _, v := range s.args {
		fmt.Fprintf(&buf, " %s", v)
	}
	fmt.Fprint(&buf, "]")
	return buf.String()
}

func consumeCallValue(p *parser) (valueType, error) {
	//grab the name identifier
	name := p.next()
	if name.typ != tokenIdent {
		return nil, fmt.Errorf("Expected a %q got a %q", tokenIdent, name)
	}
	//grab values until p.peek() is something we don't want
	values := []valueType{}
	for isBasicValueType(p.peek()) {
		//consume a basic value
		val, err := consumeBasicValue(p)
		if err != nil {
			return nil, err
		}
		//append it
		values = append(values, val)
	}
	return callValue{name.dat, values}, nil
}

// ************************
// * CONSTANT VALUE TYPES *
// ************************

// *************
// * Int Value *
// *************

type intValue int64

func (s intValue) Value(c *context) (interface{}, error) {
	return int64(s), nil
}

func (s intValue) Execute(w io.Writer, c *context) (err error) {
	val, err := s.Value(c)
	if err != nil {
		return
	}
	_, err = fmt.Fprint(w, val)
	return
}

func (s intValue) String() string {
	return fmt.Sprintf("[int %v]", int64(s))
}

func (s intValue) Byte() []byte {
	return []byte(s.String())
}

// ***************
// * Float Value *
// ***************

type floatValue float64

func (s floatValue) Value(c *context) (interface{}, error) {
	return float64(s), nil
}

func (s floatValue) Execute(w io.Writer, c *context) (err error) {
	val, err := s.Value(c)
	if err != nil {
		return
	}
	_, err = fmt.Fprint(w, val)
	return
}

func (s floatValue) String() string {
	return fmt.Sprintf("[float %f]", float64(s))
}

func (s floatValue) Byte() []byte {
	return []byte(s.String())
}

// *************************
// * String Constant Value *
// *************************

type constantValue []byte

func (s constantValue) Value(c *context) (interface{}, error) {
	return string(s), nil
}

func (s constantValue) Execute(w io.Writer, c *context) (err error) {
	val, err := s.Value(c)
	if err != nil {
		return
	}
	_, err = fmt.Fprint(w, val)
	return
}

func (s constantValue) String() string {
	return fmt.Sprintf("[constant %q]", string(s))
}

func (s *constantValue) Append(p []byte) {
	*s = append(*s, p...)
}
